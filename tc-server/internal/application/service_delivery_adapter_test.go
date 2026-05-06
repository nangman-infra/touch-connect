package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	"github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/memory"
)

const deliveryBridgeEndpointRef = "tc://endpoint/worker_delivery_bridge"

func TestServiceDeliveryAdapterBridgePublishesFetchesAndClaims(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)

	ingress, err := service.IngressMessage(deliveryBridgeMessageRequest("tc://message/msg_delivery_bridge_claim"))
	if err != nil {
		t.Fatalf("ingress message: %v", err)
	}
	if len(delivery.published) != 1 {
		t.Fatalf("expected one published delivery, got %+v", delivery.published)
	}
	if delivery.published[0].MessageRef != ingress.MessageRef || delivery.published[0].DeliveryRef != ingress.DeliveryRef {
		t.Fatalf("expected publish to preserve public refs, ingress=%+v published=%+v", ingress, delivery.published[0])
	}

	claimed, err := service.ClaimNextMessage(contracts.ClaimNextMessageRequest{EndpointRef: deliveryBridgeEndpointRef})
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claimed.Empty || claimed.Claim == nil {
		t.Fatalf("expected claim from delivery adapter bridge, got %+v", claimed)
	}
	if claimed.Claim.MessageRef != ingress.MessageRef {
		t.Fatalf("expected claim for ingressed message, got %+v", claimed.Claim)
	}
	if claimed.Claim.AttemptRef != "tc://attempt/att_000001" {
		t.Fatalf("expected processing ledger to allocate attempt ref, got %q", claimed.Claim.AttemptRef)
	}
	if len(delivery.fetchRequests) != 1 {
		t.Fatalf("expected one delivery fetch request, got %+v", delivery.fetchRequests)
	}
	if delivery.fetchRequests[0].EndpointRef != deliveryBridgeEndpointRef || len(delivery.fetchRequests[0].Capabilities) != 1 || delivery.fetchRequests[0].Capabilities[0] != "code.change" {
		t.Fatalf("expected fetch to use endpoint capability claim, got %+v", delivery.fetchRequests[0])
	}
	if len(delivery.acked) != 0 || len(delivery.nacked) != 0 {
		t.Fatalf("expected claim to avoid broker ack/nak before checkpoint, acked=%+v nacked=%+v", delivery.acked, delivery.nacked)
	}
}

func TestNewServiceValidatesPortsAndQualityError(t *testing.T) {
	store := memory.NewStore()
	if _, err := application.NewService(application.ServicePorts{}, application.DefaultSettings()); err == nil {
		t.Fatal("expected empty ports to fail")
	}
	if _, err := application.NewService(application.PortsFromStore(store), application.DefaultSettings()); err != nil {
		t.Fatalf("valid ports rejected: %v", err)
	}
	qualityErr := application.QualityRejectedError{Decision: contracts.QualityDecision{QualityDecisionRef: "tc://quality-decision/q"}}
	if qualityErr.Error() == "" {
		t.Fatal("quality error should have message")
	}
}

func TestServiceDeliveryAdapterBridgeAcksTerminalCheckpoint(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)

	ingress, err := service.IngressMessage(deliveryBridgeMessageRequest("tc://message/msg_delivery_bridge_ack"))
	if err != nil {
		t.Fatalf("ingress message: %v", err)
	}
	claimed, err := service.ClaimNextMessage(contracts.ClaimNextMessageRequest{EndpointRef: deliveryBridgeEndpointRef})
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claimed.Claim == nil {
		t.Fatalf("expected claim, got %+v", claimed)
	}

	checkpoint, err := service.SubmitCheckpoint(claimed.Claim.AttemptRef, contracts.CheckpointRequest{
		EndpointRef: deliveryBridgeEndpointRef,
		State:       domain.AttemptStateCompleted,
		Summary:     "completed through delivery adapter bridge",
	})
	if err != nil {
		t.Fatalf("submit terminal checkpoint: %v", err)
	}
	if checkpoint.State != domain.AttemptStateCompleted {
		t.Fatalf("expected completed checkpoint, got %+v", checkpoint)
	}
	if len(delivery.acked) != 1 || delivery.acked[0] != ingress.DeliveryRef {
		t.Fatalf("expected terminal checkpoint to ack delivery %q, got %+v", ingress.DeliveryRef, delivery.acked)
	}
	if len(delivery.nacked) != 0 {
		t.Fatalf("expected no nak on completed checkpoint, got %+v", delivery.nacked)
	}
}

func TestServiceDeliveryAdapterBridgeNaksWhenDomainClaimFails(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)
	delivery.queue = append(delivery.queue, application.DeliveryRecord{
		DeliveryRef: "tc://delivery/dlv_missing_domain_message",
		MessageRef:  "tc://message/msg_missing_domain_message",
		Subject:     "tc.messages.code.change",
	})

	_, err := service.ClaimNextMessage(contracts.ClaimNextMessageRequest{EndpointRef: deliveryBridgeEndpointRef})
	if !errors.Is(err, domain.ErrMessageNotFound) {
		t.Fatalf("expected ErrMessageNotFound, got %v", err)
	}
	if len(delivery.nacked) != 1 {
		t.Fatalf("expected failed domain claim to nak delivery, got %+v", delivery.nacked)
	}
	if delivery.nacked[0].deliveryRef != "tc://delivery/dlv_missing_domain_message" {
		t.Fatalf("expected missing message delivery to be nacked, got %+v", delivery.nacked[0])
	}
	if len(delivery.acked) != 0 {
		t.Fatalf("expected no ack on failed domain claim, got %+v", delivery.acked)
	}
}

func TestServiceRecordsRejectedQualityDecisionWithoutDispatch(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)
	req := deliveryBridgeMessageRequest("tc://message/msg_delivery_bridge_rejected")
	req.PhraseologyPolicy = &contracts.PhraseologyPolicy{
		PolicyRef:      "tc://quality-policy/rejecting",
		PolicyVersion:  "1",
		ScopeKind:      "task",
		RequiredFields: []string{"constraints"},
		FallbackAction: contracts.QualityFallbackReject,
		Severity:       contracts.QualitySeverityBlocking,
	}

	var qualityErr application.QualityRejectedError
	if _, err := service.IngressMessage(req); !errors.As(err, &qualityErr) {
		t.Fatalf("expected rejected quality decision to reject ingress, got %v", err)
	} else if qualityErr.Decision.QualityDecisionRef == "" {
		t.Fatalf("expected quality-aware error to carry decision ref, got %+v", qualityErr.Decision)
	}
	snapshot := service.Snapshot()
	if len(snapshot.Messages) != 0 {
		t.Fatalf("expected rejected message not to be dispatched, got %+v", snapshot.Messages)
	}
	if len(snapshot.QualityDecisions) != 1 || snapshot.QualityDecisions[0].Decision != contracts.QualityDecisionRejected {
		t.Fatalf("expected append-only rejected quality decision, got %+v", snapshot.QualityDecisions)
	}
	if len(delivery.published) != 0 {
		t.Fatalf("expected rejected quality decision not to publish delivery, got %+v", delivery.published)
	}
}

func TestServiceQualityWarnGateDispatchesAndRecordsWarnedDecision(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)
	req := rejectingQualityRequest("tc://message/msg_delivery_bridge_warn_gate")
	req.QualityGate = contracts.QualityGateWarn

	ingress, err := service.IngressMessage(req)
	if err != nil {
		t.Fatalf("expected warn gate to accept ingress, got %v", err)
	}
	if ingress.QualityDecisionRef == "" {
		t.Fatalf("expected response to expose quality decision ref, got %+v", ingress)
	}
	snapshot := service.Snapshot()
	if len(snapshot.Messages) != 1 {
		t.Fatalf("expected warned message to be dispatched, got %+v", snapshot.Messages)
	}
	if len(snapshot.QualityDecisions) != 1 || snapshot.QualityDecisions[0].Decision != contracts.QualityDecisionWarned {
		t.Fatalf("expected warned quality decision, got %+v", snapshot.QualityDecisions)
	}
	if len(delivery.published) != 1 {
		t.Fatalf("expected warn gate to publish delivery, got %+v", delivery.published)
	}
}

func TestServiceQualitySkipGateDispatchesAndRecordsSkippedDecision(t *testing.T) {
	service, delivery := newServiceWithFakeDeliveryAdapter(t)
	registerDeliveryBridgeEndpoint(t, service)
	req := rejectingQualityRequest("tc://message/msg_delivery_bridge_skip_gate")
	req.QualityGate = contracts.QualityGateSkip

	ingress, err := service.IngressMessage(req)
	if err != nil {
		t.Fatalf("expected skip gate to accept ingress, got %v", err)
	}
	if ingress.QualityDecisionRef == "" {
		t.Fatalf("expected response to expose quality decision ref, got %+v", ingress)
	}
	snapshot := service.Snapshot()
	if len(snapshot.Messages) != 1 {
		t.Fatalf("expected skipped message to be dispatched, got %+v", snapshot.Messages)
	}
	if len(snapshot.QualityDecisions) != 1 || snapshot.QualityDecisions[0].Decision != contracts.QualityDecisionSkipped {
		t.Fatalf("expected skipped quality decision, got %+v", snapshot.QualityDecisions)
	}
	if len(snapshot.QualityDecisions[0].Violations) != 0 {
		t.Fatalf("expected skip gate to avoid validator violations, got %+v", snapshot.QualityDecisions[0])
	}
	if len(delivery.published) != 1 {
		t.Fatalf("expected skip gate to publish delivery, got %+v", delivery.published)
	}
}

func newServiceWithFakeDeliveryAdapter(t *testing.T) (*application.Service, *fakeDeliveryAdapter) {
	t.Helper()
	store := memory.NewStore()
	delivery := &fakeDeliveryAdapter{}
	settings := application.DefaultSettings()
	now := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	settings.Now = func() time.Time { return now }
	service, err := application.NewServiceWithDeliveryAdapter(application.PortsFromStore(store), delivery, settings)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return service, delivery
}

func registerDeliveryBridgeEndpoint(t *testing.T, service *application.Service) {
	t.Helper()
	_, err := service.RegisterEndpoint(contracts.EndpointRegistrationRequest{
		EndpointRef:     deliveryBridgeEndpointRef,
		DisplayName:     "delivery bridge worker",
		ActorID:         "actor.delivery_bridge",
		WorkspaceID:     "workspace.delivery_bridge",
		ConnectionState: domain.EndpointStateOnline,
		Capabilities:    []contracts.Capability{{Name: "code.change"}},
		WorkerVersion:   "0.1.0-dev",
		StartedAt:       "2026-05-05T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("register endpoint: %v", err)
	}
}

func deliveryBridgeMessageRequest(messageRef string) contracts.MessageIngressRequest {
	return contracts.MessageIngressRequest{
		MessageRef:        messageRef,
		SenderEndpointRef: "tc://endpoint/control_delivery_bridge",
		TargetCapability:  "code.change",
		Payload: contracts.Payload{
			Summary:    "delivery bridge change",
			Body:       "apply the delivery bridge test change",
			References: []contracts.Reference{},
		},
		Constraints:      []contracts.Constraint{},
		CorrelationRef:   "tc://task/tsk_delivery_bridge",
		ReadbackRequired: true,
	}
}

func rejectingQualityRequest(messageRef string) contracts.MessageIngressRequest {
	req := deliveryBridgeMessageRequest(messageRef)
	req.PhraseologyPolicy = &contracts.PhraseologyPolicy{
		PolicyRef:      "tc://quality-policy/rejecting",
		PolicyVersion:  "1",
		ScopeKind:      "task",
		RequiredFields: []string{"constraints"},
		FallbackAction: contracts.QualityFallbackReject,
		Severity:       contracts.QualitySeverityBlocking,
	}
	return req
}

type fakeDeliveryAdapter struct {
	published     []domain.Message
	queue         []application.DeliveryRecord
	fetchRequests []application.DeliveryFetchRequest
	acked         []string
	nacked        []fakeNak
}

type fakeNak struct {
	deliveryRef string
	reason      string
}

func (f *fakeDeliveryAdapter) PublishAcceptedMessage(message domain.Message) (application.DeliveryReceipt, error) {
	f.published = append(f.published, message)
	f.queue = append(f.queue, application.DeliveryRecord{
		DeliveryRef: message.DeliveryRef,
		MessageRef:  message.MessageRef,
		Subject:     "tc.messages." + message.TargetCapability,
		Metadata: map[string]string{
			"tc_message_ref":  message.MessageRef,
			"tc_delivery_ref": message.DeliveryRef,
		},
	})
	return application.DeliveryReceipt{DeliveryRef: message.DeliveryRef}, nil
}

func (f *fakeDeliveryAdapter) FetchNextDelivery(request application.DeliveryFetchRequest) (application.DeliveryRecord, bool, error) {
	f.fetchRequests = append(f.fetchRequests, request)
	if len(f.queue) == 0 {
		return application.DeliveryRecord{}, false, nil
	}
	record := f.queue[0]
	f.queue = f.queue[1:]
	return record, true, nil
}

func (f *fakeDeliveryAdapter) AckDelivery(deliveryRef string) error {
	f.acked = append(f.acked, deliveryRef)
	return nil
}

func (f *fakeDeliveryAdapter) NakDelivery(deliveryRef string, reason string) error {
	f.nacked = append(f.nacked, fakeNak{deliveryRef: deliveryRef, reason: reason})
	return nil
}
