package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nats-io/nats.go"
)

const (
	defaultStreamName      = "TOUCH_CONNECT_MESSAGES"
	defaultSubjectPrefix   = "tc.messages"
	defaultConsumerName    = "touch-connect-delivery"
	defaultConnectTimeout  = 2 * time.Second
	defaultRequestTimeout  = 2 * time.Second
	defaultDuplicateWindow = 2 * time.Minute
	defaultFetchBatchSize  = 1
	defaultAckWait         = 30 * time.Second
	defaultMaxDeliver      = 5

	HeaderMessageRef     = "tc_message_ref"
	HeaderDeliveryRef    = "tc_delivery_ref"
	HeaderAttemptRef     = "tc_attempt_ref"
	HeaderCorrelationRef = "tc_correlation_ref"
	HeaderCapability     = "tc_capability"
)

var (
	ErrURLRequired               = errors.New("jetstream url is required")
	ErrInvalidStreamName         = errors.New("jetstream stream name is invalid")
	ErrInvalidConsumerName       = errors.New("jetstream consumer name is invalid")
	ErrMessageRefRequired        = errors.New("message_ref is required")
	ErrDeliveryRefRequired       = errors.New("delivery_ref is required")
	ErrFetchRequiresPullConsumer = errors.New("jetstream delivery fetch requires pull consumer binding")
	ErrAckRequiresDeliveryState  = errors.New("jetstream delivery ack requires fetched delivery state")
	ErrNakRequiresDeliveryState  = errors.New("jetstream delivery nak requires fetched delivery state")
	ErrDeliveryAlreadyPending    = errors.New("jetstream delivery is already pending")
	ErrDeliveryNotPending        = errors.New("jetstream delivery is not pending")
)

type Config struct {
	URL             string
	StreamName      string
	SubjectPrefix   string
	ConsumerName    string
	ConnectTimeout  time.Duration
	RequestTimeout  time.Duration
	DuplicateWindow time.Duration
	FetchBatchSize  int
	AckWait         time.Duration
	MaxDeliver      int
}

type Adapter struct {
	config       Config
	conn         *nats.Conn
	js           nats.JetStreamContext
	subscription *nats.Subscription
	pendingMu    sync.Mutex
	pending      map[string]*nats.Msg
}

var _ application.DeliveryAdapter = (*Adapter)(nil)

func NewAdapter(ctx context.Context, config Config) (*Adapter, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	accepted, err := config.validated()
	if err != nil {
		return nil, err
	}
	conn, err := nats.Connect(
		accepted.URL,
		nats.Name("touch-connect-jetstream-adapter"),
		nats.Timeout(accepted.ConnectTimeout),
	)
	if err != nil {
		return nil, err
	}
	js, err := conn.JetStream(nats.MaxWait(accepted.RequestTimeout))
	if err != nil {
		conn.Close()
		return nil, err
	}
	adapter := &Adapter{config: accepted, conn: conn, js: js, pending: map[string]*nats.Msg{}}
	if err := adapter.ensureStream(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	if err := adapter.ensureConsumer(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	if err := adapter.bindPullConsumer(); err != nil {
		conn.Close()
		return nil, err
	}
	return adapter, nil
}

func (a *Adapter) Close() {
	if a == nil || a.conn == nil {
		return
	}
	if a.subscription != nil {
		_ = a.subscription.Unsubscribe()
	}
	a.conn.Close()
}

func (a *Adapter) PublishAcceptedMessage(message domain.Message) (application.DeliveryReceipt, error) {
	if message.MessageRef == "" {
		return application.DeliveryReceipt{}, ErrMessageRefRequired
	}
	if message.DeliveryRef == "" {
		return application.DeliveryReceipt{}, ErrDeliveryRefRequired
	}
	data, err := json.Marshal(message)
	if err != nil {
		return application.DeliveryReceipt{}, err
	}
	subject := a.subjectForCapability(message.TargetCapability)
	headers := nats.Header{}
	headers.Set(nats.MsgIdHdr, message.MessageRef)
	headers.Set(HeaderMessageRef, message.MessageRef)
	headers.Set(HeaderDeliveryRef, message.DeliveryRef)
	headers.Set(HeaderCapability, message.TargetCapability)
	if message.AttemptRef != "" {
		headers.Set(HeaderAttemptRef, message.AttemptRef)
	}
	if message.CorrelationRef != "" {
		headers.Set(HeaderCorrelationRef, message.CorrelationRef)
	}
	ack, err := a.js.PublishMsg(&nats.Msg{
		Subject: subject,
		Header:  headers,
		Data:    data,
	}, nats.MsgId(message.MessageRef))
	if err != nil {
		return application.DeliveryReceipt{}, err
	}
	metadata := map[string]string{
		HeaderMessageRef:     message.MessageRef,
		HeaderDeliveryRef:    message.DeliveryRef,
		HeaderCapability:     message.TargetCapability,
		"adapter_stream":     ack.Stream,
		"adapter_stream_seq": strconv.FormatUint(ack.Sequence, 10),
		"adapter_duplicate":  strconv.FormatBool(ack.Duplicate),
		"adapter_subject":    subject,
	}
	if message.AttemptRef != "" {
		metadata[HeaderAttemptRef] = message.AttemptRef
	}
	if message.CorrelationRef != "" {
		metadata[HeaderCorrelationRef] = message.CorrelationRef
	}
	return application.DeliveryReceipt{DeliveryRef: message.DeliveryRef, Metadata: metadata}, nil
}

func (a *Adapter) FetchNextDelivery(request application.DeliveryFetchRequest) (application.DeliveryRecord, bool, error) {
	if a.subscription == nil {
		return application.DeliveryRecord{}, false, ErrFetchRequiresPullConsumer
	}
	allowedSubjects := a.subjectsForCapabilities(request.Capabilities)
	if len(allowedSubjects) == 0 {
		return application.DeliveryRecord{}, false, nil
	}
	messages, err := a.subscription.Fetch(a.config.FetchBatchSize, nats.MaxWait(a.config.RequestTimeout))
	if errors.Is(err, nats.ErrTimeout) {
		return application.DeliveryRecord{}, false, nil
	}
	if err != nil {
		return application.DeliveryRecord{}, false, err
	}
	for _, message := range messages {
		if !allowedSubjects[message.Subject] {
			if nakErr := message.Nak(); nakErr != nil {
				return application.DeliveryRecord{}, false, nakErr
			}
			continue
		}
		record := a.deliveryRecordFromMessage(message)
		if record.DeliveryRef == "" {
			if nakErr := message.Nak(); nakErr != nil {
				return application.DeliveryRecord{}, false, nakErr
			}
			return application.DeliveryRecord{}, false, ErrDeliveryRefRequired
		}
		if record.MessageRef == "" {
			if nakErr := message.Nak(); nakErr != nil {
				return application.DeliveryRecord{}, false, nakErr
			}
			return application.DeliveryRecord{}, false, ErrMessageRefRequired
		}
		if err := a.trackPendingDelivery(record.DeliveryRef, message); err != nil {
			if nakErr := message.Nak(); nakErr != nil {
				return application.DeliveryRecord{}, false, nakErr
			}
			return application.DeliveryRecord{}, false, err
		}
		return record, true, nil
	}
	return application.DeliveryRecord{}, false, nil
}

func (a *Adapter) AckDelivery(deliveryRef string) error {
	if deliveryRef == "" {
		return ErrDeliveryRefRequired
	}
	message, ok := a.pendingDelivery(deliveryRef)
	if !ok {
		return ErrDeliveryNotPending
	}
	if err := message.Ack(); err != nil {
		return err
	}
	a.removePendingDelivery(deliveryRef)
	return nil
}

func (a *Adapter) NakDelivery(deliveryRef string, reason string) error {
	if deliveryRef == "" {
		return ErrDeliveryRefRequired
	}
	_ = reason
	message, ok := a.pendingDelivery(deliveryRef)
	if !ok {
		return ErrDeliveryNotPending
	}
	if err := message.Nak(); err != nil {
		return err
	}
	a.removePendingDelivery(deliveryRef)
	return nil
}

func (a *Adapter) ensureStream(ctx context.Context) error {
	cfg := &nats.StreamConfig{
		Name:       a.config.StreamName,
		Subjects:   []string{a.config.SubjectPrefix + ".>"},
		Retention:  nats.WorkQueuePolicy,
		Storage:    nats.FileStorage,
		Duplicates: a.config.DuplicateWindow,
	}
	if _, err := a.js.StreamInfo(a.config.StreamName, nats.Context(ctx)); err == nil {
		return nil
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}
	_, err := a.js.AddStream(cfg, nats.Context(ctx))
	return err
}

func (a *Adapter) ensureConsumer(ctx context.Context) error {
	cfg := &nats.ConsumerConfig{
		Durable:         a.config.ConsumerName,
		Name:            a.config.ConsumerName,
		AckPolicy:       nats.AckExplicitPolicy,
		AckWait:         a.config.AckWait,
		MaxDeliver:      a.config.MaxDeliver,
		FilterSubject:   a.config.SubjectPrefix + ".>",
		MaxRequestBatch: a.config.FetchBatchSize,
	}
	_, err := a.js.AddConsumer(a.config.StreamName, cfg, nats.Context(ctx))
	return err
}

func (a *Adapter) bindPullConsumer() error {
	subscription, err := a.js.PullSubscribe(
		a.config.SubjectPrefix+".>",
		a.config.ConsumerName,
		nats.Bind(a.config.StreamName, a.config.ConsumerName),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}
	a.subscription = subscription
	return nil
}

func (a *Adapter) subjectForCapability(capability string) string {
	return a.config.SubjectPrefix + "." + sanitizeSubjectToken(capability)
}

func (a *Adapter) subjectsForCapabilities(capabilities []string) map[string]bool {
	subjects := map[string]bool{}
	for _, capability := range capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			continue
		}
		subjects[a.subjectForCapability(capability)] = true
	}
	return subjects
}

func (a *Adapter) deliveryRecordFromMessage(message *nats.Msg) application.DeliveryRecord {
	metadata := map[string]string{
		"adapter_subject": message.Subject,
	}
	copyHeader(metadata, message.Header, HeaderMessageRef)
	copyHeader(metadata, message.Header, HeaderDeliveryRef)
	copyHeader(metadata, message.Header, HeaderAttemptRef)
	copyHeader(metadata, message.Header, HeaderCorrelationRef)
	copyHeader(metadata, message.Header, HeaderCapability)
	if msgMetadata, err := message.Metadata(); err == nil {
		metadata["adapter_stream"] = msgMetadata.Stream
		metadata["adapter_consumer"] = msgMetadata.Consumer
		metadata["adapter_stream_seq"] = strconv.FormatUint(msgMetadata.Sequence.Stream, 10)
		metadata["adapter_consumer_seq"] = strconv.FormatUint(msgMetadata.Sequence.Consumer, 10)
		metadata["adapter_num_delivered"] = strconv.FormatUint(msgMetadata.NumDelivered, 10)
		metadata["adapter_num_pending"] = strconv.FormatUint(msgMetadata.NumPending, 10)
	}
	return application.DeliveryRecord{
		DeliveryRef: message.Header.Get(HeaderDeliveryRef),
		MessageRef:  message.Header.Get(HeaderMessageRef),
		Subject:     message.Subject,
		Metadata:    metadata,
	}
}

func copyHeader(metadata map[string]string, headers nats.Header, key string) {
	if value := headers.Get(key); value != "" {
		metadata[key] = value
	}
}

func (a *Adapter) trackPendingDelivery(deliveryRef string, message *nats.Msg) error {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	if a.pending == nil {
		a.pending = map[string]*nats.Msg{}
	}
	if _, ok := a.pending[deliveryRef]; ok {
		return ErrDeliveryAlreadyPending
	}
	a.pending[deliveryRef] = message
	return nil
}

func (a *Adapter) pendingDelivery(deliveryRef string) (*nats.Msg, bool) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	if a.pending == nil {
		return nil, false
	}
	message, ok := a.pending[deliveryRef]
	return message, ok
}

func (a *Adapter) removePendingDelivery(deliveryRef string) {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	delete(a.pending, deliveryRef)
}

func (c Config) validated() (Config, error) {
	c.URL = strings.TrimSpace(c.URL)
	if c.URL == "" {
		return Config{}, ErrURLRequired
	}
	c.StreamName = strings.TrimSpace(c.StreamName)
	if c.StreamName == "" {
		c.StreamName = defaultStreamName
	}
	if !validStreamName(c.StreamName) {
		return Config{}, ErrInvalidStreamName
	}
	c.ConsumerName = strings.TrimSpace(c.ConsumerName)
	if c.ConsumerName == "" {
		c.ConsumerName = defaultConsumerName
	}
	if !validStreamName(c.ConsumerName) {
		return Config{}, ErrInvalidConsumerName
	}
	c.SubjectPrefix = strings.Trim(strings.TrimSpace(c.SubjectPrefix), ".")
	if c.SubjectPrefix == "" {
		c.SubjectPrefix = defaultSubjectPrefix
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = defaultConnectTimeout
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = defaultRequestTimeout
	}
	if c.DuplicateWindow <= 0 {
		c.DuplicateWindow = defaultDuplicateWindow
	}
	if c.FetchBatchSize <= 0 {
		c.FetchBatchSize = defaultFetchBatchSize
	}
	if c.AckWait <= 0 {
		c.AckWait = defaultAckWait
	}
	if c.MaxDeliver <= 0 {
		c.MaxDeliver = defaultMaxDeliver
	}
	return c, nil
}

func validStreamName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return false
		}
		switch r {
		case '.', '*', '>', '/', '\\':
			return false
		}
	}
	return true
}

func sanitizeSubjectToken(value string) string {
	value = strings.Trim(strings.TrimSpace(value), ".")
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	lastDot := false
	for _, r := range value {
		switch {
		case r == '.':
			if !lastDot && builder.Len() > 0 {
				builder.WriteRune(r)
				lastDot = true
			}
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDot = false
		default:
			if !lastDot && builder.Len() > 0 {
				builder.WriteRune('_')
				lastDot = false
			}
		}
	}
	out := strings.Trim(builder.String(), "._")
	if out == "" {
		return "unknown"
	}
	return out
}
