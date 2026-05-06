package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-control/internal/application"
)

type Handler struct {
	service *application.Service
}

func NewHandler(service *application.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	switch r.Method {
	case http.MethodGet:
		h.serveGet(w, r, path)
	case http.MethodPost:
		h.servePost(w, r, path)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) serveGet(w http.ResponseWriter, r *http.Request, path string) {
	switch {
	case path == "healthz":
		writeJSON(w, http.StatusOK, h.service.Health())
	case path == "readyz":
		h.ready(w, r)
	case path == "version":
		h.version(w, r)
	case path == "v1/snapshot":
		h.snapshot(w, r)
	case path == "v1/endpoints":
		h.endpoints(w, r)
	case path == "v1/endpoints/inspect":
		h.endpoint(w, r)
	case path == "v1/capabilities":
		h.capabilities(w, r)
	case h.serveGetMessages(w, r, path):
	case h.serveGetTasks(w, r, path):
	case h.serveGetArtifacts(w, r, path):
	case h.serveGetApprovals(w, r, path):
	case h.serveGetDLQ(w, r, path):
	case path == "v1/side-effects":
		h.sideEffects(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) serveGetMessages(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case path == "v1/messages":
		h.messages(w, r)
	case path == "v1/messages/inspect":
		h.message(w, r)
	case path == "v1/messages/history":
		h.messages(w, r)
	default:
		return false
	}
	return true
}

func (h *Handler) serveGetTasks(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case path == "v1/tasks/status":
		h.taskStatus(w, r)
	case path == "v1/tasks/history":
		h.taskHistory(w, r)
	default:
		return false
	}
	return true
}

func (h *Handler) serveGetArtifacts(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case path == "v1/artifacts":
		h.artifacts(w, r)
	case path == "v1/artifacts/lineage":
		h.artifactLineage(w, r)
	case path == "v1/artifacts/inspect":
		h.artifact(w, r)
	default:
		return false
	}
	return true
}

func (h *Handler) serveGetApprovals(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case path == "v1/approvals":
		h.approvals(w, r)
	case path == "v1/approvals/chain":
		h.approvalChain(w, r)
	case path == "v1/approvals/inspect":
		h.approval(w, r)
	default:
		return false
	}
	return true
}

func (h *Handler) serveGetDLQ(w http.ResponseWriter, r *http.Request, path string) bool {
	switch {
	case path == "v1/dlq":
		h.deadLetters(w, r)
	case path == "v1/dlq/inspect":
		h.deadLetter(w, r)
	default:
		return false
	}
	return true
}

func (h *Handler) servePost(w http.ResponseWriter, r *http.Request, path string) {
	switch path {
	case "v1/messages":
		h.sendMessage(w, r)
	case "v1/tasks/cancel":
		h.cancelTask(w, r)
	case "v1/tasks/retry":
		h.retryTask(w, r)
	case "v1/artifacts/finalize":
		h.finalizeArtifact(w, r)
	case "v1/approvals/decide":
		h.recordApproval(w, r)
	case "v1/dlq/replay":
		h.replayDeadLetter(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Ready(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) version(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Version(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Snapshot(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) endpoints(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Endpoints(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) endpoint(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.Endpoint(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) capabilities(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Capabilities(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) messages(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Messages(r.Context(), r.URL.Query().Get("task"))
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) message(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.Message(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	var req contracts.MessageIngressRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.SendMessage(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) taskStatus(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.TaskStatus(r.Context(), r.URL.Query().Get("task"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) taskHistory(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.TaskHistory(r.Context(), r.URL.Query().Get("task"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) cancelTask(w http.ResponseWriter, r *http.Request) {
	var req contracts.TaskCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.CancelTask(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) retryTask(w http.ResponseWriter, r *http.Request) {
	var req contracts.TaskCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.RetryTask(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) artifacts(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Artifacts(r.Context(), r.URL.Query().Get("task"))
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) artifact(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.Artifact(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) artifactLineage(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.ArtifactLineage(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) finalizeArtifact(w http.ResponseWriter, r *http.Request) {
	var req contracts.ArtifactFinalizeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.FinalizeArtifact(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) approvals(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.Approvals(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) approval(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.Approval(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) approvalChain(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.ApprovalChain(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) recordApproval(w http.ResponseWriter, r *http.Request) {
	var req contracts.ApprovalCommandRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.RecordApproval(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) deadLetters(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.DeadLetters(r.Context())
	writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) deadLetter(w http.ResponseWriter, r *http.Request) {
	value, ok, err := h.service.DeadLetter(r.Context(), r.URL.Query().Get("ref"))
	writeLookupResult(w, value, ok, err)
}

func (h *Handler) replayDeadLetter(w http.ResponseWriter, r *http.Request) {
	var req contracts.DLQReplayRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	value, err := h.service.ReplayDeadLetter(r.Context(), req)
	writeResult(w, http.StatusAccepted, value, err)
}

func (h *Handler) sideEffects(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.SideEffects(r.Context(), r.URL.Query().Get("task"))
	writeResult(w, http.StatusOK, value, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid json")
		return false
	}
	return true
}

func writeLookupResult(w http.ResponseWriter, value any, ok bool, err error) {
	if err != nil {
		writeError(w, http.StatusBadGateway, "server_unavailable", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "record not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func writeResult(w http.ResponseWriter, status int, value any, err error) {
	if err != nil {
		var apiErr contracts.APIError
		if errors.As(err, &apiErr) && apiErr.Response.Code == contracts.ErrorCodeQualityRejected {
			writeErrorResponse(w, apiErr.StatusCode, apiErr.Response)
			return
		}
		writeError(w, http.StatusBadGateway, "server_unavailable", err.Error())
		return
	}
	writeJSON(w, status, value)
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
