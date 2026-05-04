package jetstream

import (
	"errors"
	"testing"
	"time"
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
