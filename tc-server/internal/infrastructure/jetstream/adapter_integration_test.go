//go:build integration && jetstream

package jetstream

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
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

func TestAdapterFetchesAndAcksDelivery(t *testing.T) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" {
		t.Skip("set NATS_URL to run JetStream adapter integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	streamName := "TC_TEST_MESSAGES_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	adapter, err := NewAdapter(ctx, Config{
		URL:            natsURL,
		StreamName:     streamName,
		SubjectPrefix:  "tc.test.messages",
		ConsumerName:   "tc-test-fetch-ack",
		RequestTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	defer adapter.Close()
	defer adapter.js.DeleteStream(streamName)

	message := domain.Message{
		MessageRef:       "tc://message/msg_jetstream_fetch_ack",
		DeliveryRef:      "tc://delivery/dlv_jetstream_fetch_ack",
		TargetCapability: "code.change",
		CorrelationRef:   "tc://task/tsk_jetstream_fetch_ack",
		State:            domain.MessageStateAvailable,
	}
	if _, err := adapter.PublishAcceptedMessage(message); err != nil {
		t.Fatalf("publish accepted message: %v", err)
	}

	record, found, err := adapter.FetchNextDelivery(applicationFetchRequest("worker-1", "code.change"))
	if err != nil {
		t.Fatalf("fetch delivery: %v", err)
	}
	if !found {
		t.Fatal("expected fetched delivery")
	}
	if record.DeliveryRef != message.DeliveryRef || record.MessageRef != message.MessageRef {
		t.Fatalf("expected public refs from headers, got %+v", record)
	}
	if record.Subject != "tc.test.messages.code.change" {
		t.Fatalf("expected capability subject, got %q", record.Subject)
	}
	if record.Metadata[HeaderCorrelationRef] != message.CorrelationRef || record.Metadata["adapter_stream"] != streamName {
		t.Fatalf("expected metadata to preserve correlation and stream, got %+v", record.Metadata)
	}
	if err := adapter.AckDelivery(record.DeliveryRef); err != nil {
		t.Fatalf("ack delivery: %v", err)
	}

	_, found, err = adapter.FetchNextDelivery(applicationFetchRequest("worker-1", "code.change"))
	if err != nil {
		t.Fatalf("fetch after ack: %v", err)
	}
	if found {
		t.Fatal("expected no delivery after ack")
	}
}

func TestAdapterNaksDeliveryForRedelivery(t *testing.T) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" {
		t.Skip("set NATS_URL to run JetStream adapter integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	streamName := "TC_TEST_MESSAGES_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	adapter, err := NewAdapter(ctx, Config{
		URL:            natsURL,
		StreamName:     streamName,
		SubjectPrefix:  "tc.test.messages",
		ConsumerName:   "tc-test-fetch-nak",
		RequestTimeout: 500 * time.Millisecond,
		MaxDeliver:     3,
	})
	if err != nil {
		t.Fatalf("create adapter: %v", err)
	}
	defer adapter.Close()
	defer adapter.js.DeleteStream(streamName)

	message := domain.Message{
		MessageRef:       "tc://message/msg_jetstream_fetch_nak",
		DeliveryRef:      "tc://delivery/dlv_jetstream_fetch_nak",
		TargetCapability: "code.change",
		State:            domain.MessageStateAvailable,
	}
	if _, err := adapter.PublishAcceptedMessage(message); err != nil {
		t.Fatalf("publish accepted message: %v", err)
	}

	first, found, err := adapter.FetchNextDelivery(applicationFetchRequest("worker-1", "code.change"))
	if err != nil {
		t.Fatalf("fetch delivery: %v", err)
	}
	if !found {
		t.Fatal("expected fetched delivery")
	}
	if err := adapter.NakDelivery(first.DeliveryRef, "test redelivery"); err != nil {
		t.Fatalf("nak delivery: %v", err)
	}

	second, found, err := adapter.FetchNextDelivery(applicationFetchRequest("worker-1", "code.change"))
	if err != nil {
		t.Fatalf("fetch after nak: %v", err)
	}
	if !found {
		t.Fatal("expected redelivered delivery")
	}
	if second.DeliveryRef != first.DeliveryRef || second.MessageRef != first.MessageRef {
		t.Fatalf("expected same public refs after redelivery, first=%+v second=%+v", first, second)
	}
	if second.Metadata["adapter_num_delivered"] != "2" {
		t.Fatalf("expected second delivery metadata, got %+v", second.Metadata)
	}
	if err := adapter.AckDelivery(second.DeliveryRef); err != nil {
		t.Fatalf("ack redelivered delivery: %v", err)
	}
}

func applicationFetchRequest(endpointRef string, capabilities ...string) application.DeliveryFetchRequest {
	return application.DeliveryFetchRequest{EndpointRef: endpointRef, Capabilities: capabilities}
}
