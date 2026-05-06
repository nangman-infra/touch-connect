package jetstream

import (
	"errors"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nats-io/nats.go"
)

func TestConfigValidatedSetsDefaults(t *testing.T) {
	accepted, err := Config{URL: " nats://127.0.0.1:4222 "}.validated()
	if err != nil {
		t.Fatalf("validate config: %v", err)
	}
	if accepted.URL != "nats://127.0.0.1:4222" {
		t.Fatalf("expected trimmed url, got %q", accepted.URL)
	}
	if accepted.StreamName != defaultStreamName {
		t.Fatalf("expected default stream name, got %q", accepted.StreamName)
	}
	if accepted.SubjectPrefix != defaultSubjectPrefix {
		t.Fatalf("expected default subject prefix, got %q", accepted.SubjectPrefix)
	}
	if accepted.ConsumerName != defaultConsumerName {
		t.Fatalf("expected default consumer name, got %q", accepted.ConsumerName)
	}
	if accepted.ConnectTimeout != defaultConnectTimeout || accepted.RequestTimeout != defaultRequestTimeout {
		t.Fatalf("expected default timeouts, got connect=%s request=%s", accepted.ConnectTimeout, accepted.RequestTimeout)
	}
	if accepted.DuplicateWindow != defaultDuplicateWindow {
		t.Fatalf("expected default duplicate window, got %s", accepted.DuplicateWindow)
	}
	if accepted.FetchBatchSize != defaultFetchBatchSize {
		t.Fatalf("expected default fetch batch size, got %d", accepted.FetchBatchSize)
	}
	if accepted.AckWait != defaultAckWait {
		t.Fatalf("expected default ack wait, got %s", accepted.AckWait)
	}
	if accepted.MaxDeliver != defaultMaxDeliver {
		t.Fatalf("expected default max deliver, got %d", accepted.MaxDeliver)
	}
}

func TestConfigValidatedRejectsMissingURL(t *testing.T) {
	if _, err := (Config{}).validated(); !errors.Is(err, ErrURLRequired) {
		t.Fatalf("expected ErrURLRequired, got %v", err)
	}
}

func TestConfigValidatedRejectsInvalidStreamName(t *testing.T) {
	if _, err := (Config{URL: "nats://127.0.0.1:4222", StreamName: "BAD.STREAM"}).validated(); !errors.Is(err, ErrInvalidStreamName) {
		t.Fatalf("expected ErrInvalidStreamName, got %v", err)
	}
}

func TestConfigValidatedRejectsInvalidConsumerName(t *testing.T) {
	if _, err := (Config{URL: "nats://127.0.0.1:4222", ConsumerName: "BAD.CONSUMER"}).validated(); !errors.Is(err, ErrInvalidConsumerName) {
		t.Fatalf("expected ErrInvalidConsumerName, got %v", err)
	}
}

func TestConfigValidatedPreservesExplicitValues(t *testing.T) {
	accepted, err := Config{
		URL:             "nats://example:4222",
		StreamName:      "CUSTOM_STREAM",
		SubjectPrefix:   ".tc.custom.",
		ConsumerName:    "custom-consumer",
		ConnectTimeout:  time.Second,
		RequestTimeout:  2 * time.Second,
		DuplicateWindow: 3 * time.Minute,
		FetchBatchSize:  10,
		AckWait:         30 * time.Second,
		MaxDeliver:      7,
	}.validated()
	if err != nil {
		t.Fatalf("validate config: %v", err)
	}
	if accepted.StreamName != "CUSTOM_STREAM" || accepted.SubjectPrefix != "tc.custom" || accepted.ConsumerName != "custom-consumer" {
		t.Fatalf("expected explicit stream and subject, got %+v", accepted)
	}
	if accepted.ConnectTimeout != time.Second || accepted.RequestTimeout != 2*time.Second || accepted.DuplicateWindow != 3*time.Minute {
		t.Fatalf("expected explicit durations, got %+v", accepted)
	}
	if accepted.FetchBatchSize != 10 || accepted.AckWait != 30*time.Second || accepted.MaxDeliver != 7 {
		t.Fatalf("expected explicit delivery settings, got %+v", accepted)
	}
}

func TestSubjectForCapabilitySanitizesUnsafeTokens(t *testing.T) {
	adapter := &Adapter{config: Config{SubjectPrefix: "tc.messages"}}
	cases := map[string]string{
		"code.change":       "tc.messages.code.change",
		" code change ":     "tc.messages.code_change",
		">":                 "tc.messages.unknown",
		"repo/write*unsafe": "tc.messages.repo_write_unsafe",
		"a..b":              "tc.messages.a.b",
	}
	for input, expected := range cases {
		if got := adapter.subjectForCapability(input); got != expected {
			t.Fatalf("subjectForCapability(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSubjectsForCapabilitiesSanitizesAndDropsBlankValues(t *testing.T) {
	adapter := &Adapter{config: Config{SubjectPrefix: "tc.messages"}}
	subjects := adapter.subjectsForCapabilities([]string{"code.change", " repo/write ", "", ">"})
	expected := map[string]bool{
		"tc.messages.code.change": true,
		"tc.messages.repo_write":  true,
		"tc.messages.unknown":     true,
	}
	if len(subjects) != len(expected) {
		t.Fatalf("expected subjects %+v, got %+v", expected, subjects)
	}
	for subject := range expected {
		if !subjects[subject] {
			t.Fatalf("missing subject %q from %+v", subject, subjects)
		}
	}
}

func TestAckAndNakRequirePendingDelivery(t *testing.T) {
	adapter := &Adapter{}
	if err := adapter.AckDelivery(""); !errors.Is(err, ErrDeliveryRefRequired) {
		t.Fatalf("expected ErrDeliveryRefRequired, got %v", err)
	}
	if err := adapter.AckDelivery("tc://delivery/missing"); !errors.Is(err, ErrDeliveryNotPending) {
		t.Fatalf("expected ErrDeliveryNotPending on ack, got %v", err)
	}
	if err := adapter.NakDelivery("", "test"); !errors.Is(err, ErrDeliveryRefRequired) {
		t.Fatalf("expected ErrDeliveryRefRequired, got %v", err)
	}
	if err := adapter.NakDelivery("tc://delivery/missing", "test"); !errors.Is(err, ErrDeliveryNotPending) {
		t.Fatalf("expected ErrDeliveryNotPending on nak, got %v", err)
	}
}

func TestPublishAcceptedMessageValidatesRefsBeforeBrokerUse(t *testing.T) {
	adapter := &Adapter{}
	if _, err := adapter.PublishAcceptedMessage(domain.Message{DeliveryRef: "tc://delivery/d"}); !errors.Is(err, ErrMessageRefRequired) {
		t.Fatalf("expected ErrMessageRefRequired, got %v", err)
	}
	if _, err := adapter.PublishAcceptedMessage(domain.Message{MessageRef: "tc://message/m"}); !errors.Is(err, ErrDeliveryRefRequired) {
		t.Fatalf("expected ErrDeliveryRefRequired, got %v", err)
	}
}

func TestFetchNextDeliveryValidatesConsumerAndCapabilities(t *testing.T) {
	adapter := &Adapter{}
	if _, _, err := adapter.FetchNextDelivery(application.DeliveryFetchRequest{Capabilities: []string{"code.change"}}); !errors.Is(err, ErrFetchRequiresPullConsumer) {
		t.Fatalf("expected ErrFetchRequiresPullConsumer, got %v", err)
	}
	adapter.subscription = &nats.Subscription{}
	if _, found, err := adapter.FetchNextDelivery(application.DeliveryFetchRequest{}); err != nil || found {
		t.Fatalf("empty capabilities should return no delivery, found=%v err=%v", found, err)
	}
}

func TestDeliveryRecordFromMessageCopiesHeadersAndMetadata(t *testing.T) {
	adapter := &Adapter{}
	message := &nats.Msg{
		Subject: "tc.messages.code.change",
		Header:  nats.Header{},
		Data:    []byte(`{"payload":"ignored"}`),
	}
	message.Header.Set(HeaderMessageRef, "tc://message/m")
	message.Header.Set(HeaderDeliveryRef, "tc://delivery/d")
	message.Header.Set(HeaderAttemptRef, "tc://attempt/a")
	message.Header.Set(HeaderCorrelationRef, "tc://task/t")
	message.Header.Set(HeaderCapability, "code.change")

	record := adapter.deliveryRecordFromMessage(message)
	if record.MessageRef != "tc://message/m" || record.DeliveryRef != "tc://delivery/d" || record.Subject != "tc.messages.code.change" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.Metadata[HeaderAttemptRef] != "tc://attempt/a" || record.Metadata[HeaderCapability] != "code.change" || record.Metadata["adapter_subject"] != "tc.messages.code.change" {
		t.Fatalf("unexpected metadata: %+v", record.Metadata)
	}
}

func TestPendingDeliveryLifecycle(t *testing.T) {
	adapter := &Adapter{}
	message := &nats.Msg{}
	if err := adapter.trackPendingDelivery("tc://delivery/d", message); err != nil {
		t.Fatalf("track pending delivery: %v", err)
	}
	if err := adapter.trackPendingDelivery("tc://delivery/d", message); !errors.Is(err, ErrDeliveryAlreadyPending) {
		t.Fatalf("expected duplicate pending error, got %v", err)
	}
	if got, ok := adapter.pendingDelivery("tc://delivery/d"); !ok || got != message {
		t.Fatal("pending delivery not found")
	}
	adapter.removePendingDelivery("tc://delivery/d")
	if _, ok := adapter.pendingDelivery("tc://delivery/d"); ok {
		t.Fatal("pending delivery should be removed")
	}
}

func TestValidateDeliveryRecord(t *testing.T) {
	if err := validateDeliveryRecord(application.DeliveryRecord{}); !errors.Is(err, ErrDeliveryRefRequired) {
		t.Fatalf("expected delivery ref error, got %v", err)
	}
	if err := validateDeliveryRecord(application.DeliveryRecord{DeliveryRef: "tc://delivery/d"}); !errors.Is(err, ErrMessageRefRequired) {
		t.Fatalf("expected message ref error, got %v", err)
	}
	if err := validateDeliveryRecord(application.DeliveryRecord{DeliveryRef: "tc://delivery/d", MessageRef: "tc://message/m"}); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
}
