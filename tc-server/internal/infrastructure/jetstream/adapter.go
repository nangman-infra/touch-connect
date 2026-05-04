package jetstream

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nats-io/nats.go"
)

const (
	defaultStreamName      = "TOUCH_CONNECT_MESSAGES"
	defaultSubjectPrefix   = "tc.messages"
	defaultConnectTimeout  = 2 * time.Second
	defaultRequestTimeout  = 2 * time.Second
	defaultDuplicateWindow = 2 * time.Minute

	HeaderMessageRef     = "tc_message_ref"
	HeaderDeliveryRef    = "tc_delivery_ref"
	HeaderAttemptRef     = "tc_attempt_ref"
	HeaderCorrelationRef = "tc_correlation_ref"
	HeaderCapability     = "tc_capability"
)

var (
	ErrURLRequired               = errors.New("jetstream url is required")
	ErrInvalidStreamName         = errors.New("jetstream stream name is invalid")
	ErrMessageRefRequired        = errors.New("message_ref is required")
	ErrDeliveryRefRequired       = errors.New("delivery_ref is required")
	ErrFetchRequiresPullConsumer = errors.New("jetstream delivery fetch requires pull consumer binding")
	ErrAckRequiresDeliveryState  = errors.New("jetstream delivery ack requires fetched delivery state")
	ErrNakRequiresDeliveryState  = errors.New("jetstream delivery nak requires fetched delivery state")
)

type Config struct {
	URL             string
	StreamName      string
	SubjectPrefix   string
	ConnectTimeout  time.Duration
	RequestTimeout  time.Duration
	DuplicateWindow time.Duration
}

type Adapter struct {
	config Config
	conn   *nats.Conn
	js     nats.JetStreamContext
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
	adapter := &Adapter{config: accepted, conn: conn, js: js}
	if err := adapter.ensureStream(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return adapter, nil
}

func (a *Adapter) Close() {
	if a == nil || a.conn == nil {
		return
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
	return application.DeliveryRecord{}, false, ErrFetchRequiresPullConsumer
}

func (a *Adapter) AckDelivery(deliveryRef string) error {
	if deliveryRef == "" {
		return ErrDeliveryRefRequired
	}
	return ErrAckRequiresDeliveryState
}

func (a *Adapter) NakDelivery(deliveryRef string, reason string) error {
	if deliveryRef == "" {
		return ErrDeliveryRefRequired
	}
	return ErrNakRequiresDeliveryState
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

func (a *Adapter) subjectForCapability(capability string) string {
	return a.config.SubjectPrefix + "." + sanitizeSubjectToken(capability)
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
