//go:build integration && jetstream

package jetstream

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nats-io/nats.go"
)

func TestAdapterPublishesAcceptedMessageWithDedupe(t *testing.T) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" {
		t.Skip("set NATS_URL to run JetStream adapter integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	streamName := "TC_TEST_MESSAGES_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	adapter, err := NewAdapter(ctx, Config{
		URL:           natsURL,
		StreamName:    streamName,
		SubjectPrefix: "tc.test.messages",
	})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	defer adapter.Close()
	defer adapter.js.DeleteStream(streamName)

	message := domain.Message{
		MessageRef:       "tc://message/msg_jetstream_publish",
		DeliveryRef:      "tc://delivery/dlv_jetstream_publish",
		TargetCapability: "code.change",
		CorrelationRef:   "tc://task/tsk_jetstream_publish",
		ReadbackRequired: true,
		State:            domain.MessageStateAvailable,
	}
	first, err := adapter.PublishAcceptedMessage(message)
	if err != nil {
		t.Fatalf("publish accepted message: %v", err)
	}
	if first.DeliveryRef != message.DeliveryRef {
		t.Fatalf("expected delivery ref %q, got %+v", message.DeliveryRef, first)
	}
	if first.Metadata["adapter_duplicate"] != "false" {
		t.Fatalf("expected first publish not to be duplicate, got %+v", first.Metadata)
	}

	second, err := adapter.PublishAcceptedMessage(message)
	if err != nil {
		t.Fatalf("publish duplicate message: %v", err)
	}
	if second.Metadata["adapter_duplicate"] != "true" {
		t.Fatalf("expected duplicate publish ack, got %+v", second.Metadata)
	}
	if second.Metadata["adapter_stream_seq"] != first.Metadata["adapter_stream_seq"] {
		t.Fatalf("expected duplicate to keep stream sequence, first=%+v second=%+v", first.Metadata, second.Metadata)
	}

	info, err := adapter.js.StreamInfo(streamName)
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("expected one stored message after duplicate publish, got %+v", info.State)
	}

	raw, err := adapter.js.GetLastMsg(streamName, "tc.test.messages.code.change")
	if err != nil {
		t.Fatalf("get last message: %v", err)
	}
	if raw.Header.Get(nats.MsgIdHdr) != message.MessageRef {
		t.Fatalf("expected Nats-Msg-Id header to preserve message_ref, got %q", raw.Header.Get(nats.MsgIdHdr))
	}
	if raw.Header.Get(HeaderMessageRef) != message.MessageRef || raw.Header.Get(HeaderDeliveryRef) != message.DeliveryRef {
		t.Fatalf("expected tc metadata headers, got %+v", raw.Header)
	}
	if raw.Header.Get(HeaderCorrelationRef) != message.CorrelationRef || raw.Header.Get(HeaderCapability) != message.TargetCapability {
		t.Fatalf("expected correlation and capability headers, got %+v", raw.Header)
	}
}
