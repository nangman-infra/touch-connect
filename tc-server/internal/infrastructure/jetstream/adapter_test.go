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
	if accepted.ConnectTimeout != defaultConnectTimeout || accepted.RequestTimeout != defaultRequestTimeout {
		t.Fatalf("expected default timeouts, got connect=%s request=%s", accepted.ConnectTimeout, accepted.RequestTimeout)
	}
	if accepted.DuplicateWindow != defaultDuplicateWindow {
		t.Fatalf("expected default duplicate window, got %s", accepted.DuplicateWindow)
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

func TestConfigValidatedPreservesExplicitValues(t *testing.T) {
	accepted, err := Config{
		URL:             "nats://example:4222",
		StreamName:      "CUSTOM_STREAM",
		SubjectPrefix:   ".tc.custom.",
		ConnectTimeout:  time.Second,
		RequestTimeout:  2 * time.Second,
		DuplicateWindow: 3 * time.Minute,
	}.validated()
	if err != nil {
		t.Fatalf("validate config: %v", err)
	}
	if accepted.StreamName != "CUSTOM_STREAM" || accepted.SubjectPrefix != "tc.custom" {
		t.Fatalf("expected explicit stream and subject, got %+v", accepted)
	}
	if accepted.ConnectTimeout != time.Second || accepted.RequestTimeout != 2*time.Second || accepted.DuplicateWindow != 3*time.Minute {
		t.Fatalf("expected explicit durations, got %+v", accepted)
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
