package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
	a2aadapter "github.com/nangman-infra/touch-connect/tc-server/internal/infrastructure/a2a"
)

type Handler struct {
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && path == "healthz":
		writeJSON(w, http.StatusOK, h.service.Health())
	case r.Method == http.MethodGet && path == "readyz":
		health := h.service.Health()
		health.Status = "ready"
		writeJSON(w, http.StatusOK, health)
	case r.Method == http.MethodGet && path == "version":
		writeJSON(w, http.StatusOK, h.service.Version())
	case r.Method == http.MethodGet && path == ".well-known/agent.json":
		h.a2aAgentCard(w, r)
	case r.Method == http.MethodPost && (path == "a2a/rpc" || path == "v1/a2a/rpc"):
		h.a2aRPC(w, r)
	case r.Method == http.MethodGet && path == "v1/control/snapshot":
		writeJSON(w, http.StatusOK, h.service.SnapshotResponse())
	case r.Method == http.MethodPost && path == "v1/control/tasks/cancel":
		h.cancelTask(w, r)
	case r.Method == http.MethodPost && path == "v1/control/tasks/retry":
		h.retryTask(w, r)
	case r.Method == http.MethodPost && path == "v1/control/dlq/replay":
		h.replayDeadLetter(w, r)
	case r.Method == http.MethodPost && path == "v1/control/artifacts/finalize":
		h.finalizeArtifact(w, r)
	case r.Method == http.MethodPost && path == "v1/endpoints/register":
		h.registerEndpoint(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/endpoints/") && strings.HasSuffix(path, "/heartbeat"):
		h.heartbeatEndpoint(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/endpoints/") && strings.HasSuffix(path, "/capabilities"):
		h.advertiseCapabilities(w, r, path)
	case r.Method == http.MethodPost && path == "v1/messages":
		h.ingressMessage(w, r)
	case r.Method == http.MethodPost && path == "v1/messages/claim-next":
		h.claimNextMessage(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/messages/") && strings.HasSuffix(path, "/claim"):
		h.claimMessage(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/checkpoints"):
		h.submitCheckpoint(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/readback"):
		h.submitReadback(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/lease"):
		h.refreshLease(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/artifacts"):
		h.registerArtifactVersion(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/approvals"):
		h.recordApprovalDecision(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/side-effects"):
		h.startSideEffectExecution(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/attempts/") && strings.HasSuffix(path, "/complete"):
		h.completeAttempt(w, r, path)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "v1/side-effects/") && strings.HasSuffix(path, "/complete"):
		h.completeSideEffectExecution(w, r, path)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) registerEndpoint(w http.ResponseWriter, r *http.Request) {
	var req contracts.EndpointRegistrationRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.RegisterEndpoint(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) advertiseCapabilities(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.CapabilityAdvertisementRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.AdvertiseCapabilities(endpointRefFromPath(path, "/capabilities"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) heartbeatEndpoint(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.EndpointHeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.HeartbeatEndpoint(endpointRefFromPath(path, "/heartbeat"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) ingressMessage(w http.ResponseWriter, r *http.Request) {
	var req contracts.MessageIngressRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.IngressMessage(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) a2aAgentCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("A2A-Version", a2aadapter.ProtocolVersion)
	card := a2aadapter.AgentCardFromSnapshot(a2aBaseURL(r), h.service.SnapshotResponse(), h.service.Version().Version)
	writeJSON(w, http.StatusOK, card)
}

func (h *Handler) a2aRPC(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("A2A-Version", a2aadapter.ProtocolVersion)
	if !a2aadapter.VersionSupported(r.Header.Get("A2A-Version")) {
		writeA2AResponse(w, a2aadapter.ErrorResponse(nil, a2aadapter.ErrorVersionUnsupported, a2aadapter.ErrVersionNotSupported.Error(), map[string]any{
			"supported_version": a2aadapter.ProtocolVersion,
		}))
		return
	}
	defer r.Body.Close()
	var rpc a2aadapter.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
		writeA2AResponse(w, a2aadapter.ErrorResponse(nil, a2aadapter.ErrorParse, "Invalid JSON payload", nil))
		return
	}
	if err := a2aadapter.ValidateJSONRPCRequest(rpc); err != nil {
		writeA2AResponse(w, a2aadapter.ErrorResponse(rpc.ID, a2aadapter.ErrorInvalidRequest, err.Error(), nil))
		return
	}
	switch rpc.Method {
	case a2aadapter.MethodSendMessage:
		h.a2aSendMessage(w, rpc)
	case a2aadapter.MethodGetTask:
		h.a2aGetTask(w, rpc)
	default:
		writeA2AResponse(w, a2aadapter.ErrorResponse(rpc.ID, a2aadapter.ErrorMethodNotFound, a2aadapter.ErrUnsupportedA2AMethod.Error(), map[string]any{
			"method": rpc.Method,
		}))
	}
}

func (h *Handler) a2aSendMessage(w http.ResponseWriter, rpc a2aadapter.JSONRPCRequest) {
	req, err := a2aadapter.DecodeSendMessageRequest(rpc.Params)
	if err != nil {
		writeA2AError(w, rpc.ID, err)
		return
	}
	ingress, err := a2aadapter.MessageIngressRequest(req)
	if err != nil {
		writeA2AError(w, rpc.ID, err)
		return
	}
	accepted, err := h.service.IngressMessage(ingress)
	if err != nil {
		writeA2AError(w, rpc.ID, err)
		return
	}
	writeA2AResponse(w, a2aadapter.ResultResponse(rpc.ID, a2aadapter.SendMessageResponseFromIngress(req, accepted)))
}

func (h *Handler) a2aGetTask(w http.ResponseWriter, rpc a2aadapter.JSONRPCRequest) {
	req, err := a2aadapter.DecodeGetTaskRequest(rpc.Params)
	if err != nil {
		writeA2AError(w, rpc.ID, err)
		return
	}
	task, ok := a2aadapter.TaskFromSnapshot(req.ID, h.service.SnapshotResponse())
	if !ok {
		writeA2AError(w, rpc.ID, a2aadapter.ErrTaskNotFound)
		return
	}
	writeA2AResponse(w, a2aadapter.ResultResponse(rpc.ID, task))
}

func (h *Handler) cancelTask(w http.ResponseWriter, r *http.Request) {
	var req contracts.TaskCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.CancelTask(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) retryTask(w http.ResponseWriter, r *http.Request) {
	var req contracts.TaskCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.RetryTask(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) replayDeadLetter(w http.ResponseWriter, r *http.Request) {
	var req contracts.DLQReplayRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.ReplayDeadLetter(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) finalizeArtifact(w http.ResponseWriter, r *http.Request) {
	var req contracts.ArtifactFinalizeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.FinalizeArtifact(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) claimMessage(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.ClaimMessageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.ClaimMessage(messageRefFromPath(path), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) claimNextMessage(w http.ResponseWriter, r *http.Request) {
	var req contracts.ClaimNextMessageRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.ClaimNextMessage(req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) submitCheckpoint(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.CheckpointRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.SubmitCheckpoint(attemptRefFromPath(path, "/checkpoints"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) submitReadback(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.ReadbackRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.SubmitReadback(attemptRefFromPath(path, "/readback"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) refreshLease(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.RefreshLeaseRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.RefreshLease(attemptRefFromPath(path, "/lease"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) registerArtifactVersion(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.ArtifactVersionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.RegisterArtifactVersion(attemptRefFromPath(path, "/artifacts"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) recordApprovalDecision(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.ApprovalDecisionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.RecordApprovalDecision(attemptRefFromPath(path, "/approvals"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) startSideEffectExecution(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.SideEffectExecutionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.StartSideEffectExecution(attemptRefFromPath(path, "/side-effects"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) completeAttempt(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.CompleteAttemptRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.CompleteAttempt(attemptRefFromPath(path, "/complete"), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func (h *Handler) completeSideEffectExecution(w http.ResponseWriter, r *http.Request, path string) {
	var req contracts.CompleteSideEffectExecutionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.service.CompleteSideEffectExecution(sideEffectRefFromPath(path), req)
	writeResult(w, http.StatusAccepted, res, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid json")
		return false
	}
	return true
}

func writeResult(w http.ResponseWriter, status int, value any, err error) {
	if err == nil {
		writeJSON(w, status, value)
		return
	}
	var qualityRejected application.QualityRejectedError
	if errors.As(err, &qualityRejected) {
		writeErrorResponse(w, http.StatusBadRequest, contracts.ErrorResponse{
			Code:            contracts.ErrorCodeQualityRejected,
			Message:         "message failed the enforce quality gate",
			QualityDecision: &qualityRejected.Decision,
		})
		return
	}
	if errors.Is(err, domain.ErrEndpointNotFound) ||
		errors.Is(err, domain.ErrCapabilityNotFound) ||
		errors.Is(err, domain.ErrMessageNotFound) ||
		errors.Is(err, domain.ErrAttemptNotFound) ||
		errors.Is(err, domain.ErrArtifactNotFound) ||
		errors.Is(err, domain.ErrApprovalNotFound) ||
		errors.Is(err, domain.ErrSideEffectNotFound) {
		writeError(w, http.StatusNotFound, err.Error(), err.Error())
		return
	}
	if errors.Is(err, domain.ErrMessageUnavailable) ||
		errors.Is(err, domain.ErrStaleAttempt) ||
		errors.Is(err, domain.ErrEndpointStale) ||
		errors.Is(err, domain.ErrLeaseExpired) ||
		errors.Is(err, domain.ErrMessageDeadLettered) ||
		errors.Is(err, domain.ErrArtifactExists) ||
		errors.Is(err, domain.ErrApprovalRequired) ||
		errors.Is(err, domain.ErrApprovalRejected) ||
		errors.Is(err, domain.ErrApprovalExpired) ||
		errors.Is(err, domain.ErrApprovalHashMismatch) ||
		errors.Is(err, domain.ErrSelfApproval) ||
		errors.Is(err, domain.ErrSideEffectConflict) {
		writeError(w, http.StatusConflict, err.Error(), err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error(), err.Error())
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, contracts.ErrorResponse{Code: code, Message: message})
}

func writeErrorResponse(w http.ResponseWriter, status int, response contracts.ErrorResponse) {
	writeJSON(w, status, response)
}

func writeA2AError(w http.ResponseWriter, id any, err error) {
	writeA2AResponse(w, a2aadapter.ErrorResponse(id, a2aadapter.ErrorCode(err), err.Error(), map[string]any{
		"reason": err.Error(),
	}))
}

func writeA2AResponse(w http.ResponseWriter, response a2aadapter.JSONRPCResponse) {
	writeJSON(w, http.StatusOK, response)
}

func a2aBaseURL(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if r.TLS != nil {
			scheme = "https"
		}
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host
}

func endpointRefFromPath(path string, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "v1/endpoints/"), suffix)
}

func messageRefFromPath(path string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "v1/messages/"), "/claim")
}

func attemptRefFromPath(path string, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "v1/attempts/"), suffix)
}

func sideEffectRefFromPath(path string) string {
	return strings.TrimSuffix(strings.TrimPrefix(path, "v1/side-effects/"), "/complete")
}
