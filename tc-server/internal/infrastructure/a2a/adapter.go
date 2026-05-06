package a2a

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	ProtocolVersion = "1.0"

	MethodSendMessage = "SendMessage"
	MethodGetTask     = "GetTask"

	RoleUser  = "ROLE_USER"
	RoleAgent = "ROLE_AGENT"

	TaskStateSubmitted     = "TASK_STATE_SUBMITTED"
	TaskStateWorking       = "TASK_STATE_WORKING"
	TaskStateCompleted     = "TASK_STATE_COMPLETED"
	TaskStateFailed        = "TASK_STATE_FAILED"
	TaskStateCanceled      = "TASK_STATE_CANCELED"
	TaskStateInputRequired = "TASK_STATE_INPUT_REQUIRED"
	TaskStateRejected      = "TASK_STATE_REJECTED"
	TaskStateUnknown       = "TASK_STATE_UNKNOWN"

	ErrorParse              = -32700
	ErrorInvalidRequest     = -32600
	ErrorMethodNotFound     = -32601
	ErrorInvalidParams      = -32602
	ErrorInternal           = -32603
	ErrorTaskNotFound       = -32001
	ErrorVersionUnsupported = -32002

	mediaTypeTextPlain       = "text/plain"
	mediaTypeApplicationJSON = "application/json"
)

var (
	ErrInvalidRequest       = errors.New("invalid A2A request")
	ErrInvalidParams        = errors.New("invalid A2A params")
	ErrCapabilityRequired   = errors.New("A2A metadata.target_capability is required")
	ErrMessagePartRequired  = errors.New("A2A message.parts requires at least one text, data, or url part")
	ErrTaskNotFound         = errors.New("A2A task not found")
	ErrVersionNotSupported  = errors.New("A2A version not supported")
	ErrUnsupportedA2AMethod = errors.New("A2A method not supported")
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type SendMessageRequest struct {
	Tenant        string         `json:"tenant,omitempty"`
	Message       Message        `json:"message"`
	Configuration map[string]any `json:"configuration,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type SendMessageResponse struct {
	Task    *Task    `json:"task,omitempty"`
	Message *Message `json:"message,omitempty"`
}

type GetTaskRequest struct {
	Tenant        string `json:"tenant,omitempty"`
	ID            string `json:"id"`
	HistoryLength int    `json:"historyLength,omitempty"`
}

type Message struct {
	MessageID        string         `json:"messageId"`
	ContextID        string         `json:"contextId,omitempty"`
	TaskID           string         `json:"taskId,omitempty"`
	Role             string         `json:"role"`
	Parts            []Part         `json:"parts"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Extensions       []string       `json:"extensions,omitempty"`
	ReferenceTaskIDs []string       `json:"referenceTaskIds,omitempty"`
}

type Part struct {
	Text      string         `json:"text,omitempty"`
	Raw       string         `json:"raw,omitempty"`
	URL       string         `json:"url,omitempty"`
	Data      any            `json:"data,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Filename  string         `json:"filename,omitempty"`
	MediaType string         `json:"mediaType,omitempty"`
}

type Task struct {
	ID        string         `json:"id"`
	ContextID string         `json:"contextId,omitempty"`
	Status    TaskStatus     `json:"status"`
	Artifacts []Artifact     `json:"artifacts,omitempty"`
	History   []Message      `json:"history,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type TaskStatus struct {
	State     string   `json:"state"`
	Message   *Message `json:"message,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
}

type Artifact struct {
	ArtifactID  string         `json:"artifactId"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parts       []Part         `json:"parts"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Extensions  []string       `json:"extensions,omitempty"`
}

type AgentCard struct {
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	SupportedInterfaces []AgentInterface  `json:"supportedInterfaces"`
	Provider            *AgentProvider    `json:"provider,omitempty"`
	Version             string            `json:"version"`
	DocumentationURL    string            `json:"documentationUrl,omitempty"`
	Capabilities        AgentCapabilities `json:"capabilities"`
	DefaultInputModes   []string          `json:"defaultInputModes"`
	DefaultOutputModes  []string          `json:"defaultOutputModes"`
	Skills              []AgentSkill      `json:"skills"`
	IconURL             string            `json:"iconUrl,omitempty"`
}

type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
	Tenant          string `json:"tenant,omitempty"`
}

type AgentProvider struct {
	URL          string `json:"url"`
	Organization string `json:"organization"`
}

type AgentCapabilities struct {
	Streaming         bool             `json:"streaming,omitempty"`
	PushNotifications bool             `json:"pushNotifications,omitempty"`
	Extensions        []AgentExtension `json:"extensions,omitempty"`
	ExtendedAgentCard bool             `json:"extendedAgentCard,omitempty"`
}

type AgentExtension struct {
	URI         string         `json:"uri,omitempty"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

func ValidateJSONRPCRequest(req JSONRPCRequest) error {
	if req.JSONRPC != "2.0" || strings.TrimSpace(req.Method) == "" {
		return ErrInvalidRequest
	}
	return nil
}

func VersionSupported(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return true
	}
	return version == ProtocolVersion || strings.HasPrefix(version, "1.")
}

func DecodeSendMessageRequest(raw json.RawMessage) (SendMessageRequest, error) {
	var req SendMessageRequest
	if len(raw) == 0 {
		return SendMessageRequest{}, ErrInvalidParams
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return SendMessageRequest{}, err
	}
	if strings.TrimSpace(req.Message.MessageID) == "" || len(req.Message.Parts) == 0 {
		return SendMessageRequest{}, ErrInvalidParams
	}
	if !validClientRole(req.Message.Role) {
		return SendMessageRequest{}, ErrInvalidParams
	}
	return req, nil
}

func DecodeGetTaskRequest(raw json.RawMessage) (GetTaskRequest, error) {
	var req GetTaskRequest
	if len(raw) == 0 {
		return GetTaskRequest{}, ErrInvalidParams
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return GetTaskRequest{}, err
	}
	if strings.TrimSpace(req.ID) == "" {
		return GetTaskRequest{}, ErrInvalidParams
	}
	return req, nil
}

func MessageIngressRequest(req SendMessageRequest) (contracts.MessageIngressRequest, error) {
	metadata := mergeMetadata(req.Metadata, req.Message.Metadata)
	capability := metadataString(metadata, "target_capability", "tc_target_capability", "capability")
	if capability == "" {
		return contracts.MessageIngressRequest{}, ErrCapabilityRequired
	}
	body, references, err := partsToPayload(req.Message)
	if err != nil {
		return contracts.MessageIngressRequest{}, err
	}
	qualityGate, err := qualityGateFromMetadata(metadata)
	if err != nil {
		return contracts.MessageIngressRequest{}, err
	}
	return contracts.MessageIngressRequest{
		MessageRef:        metadataString(metadata, "tc_message_ref", "message_ref"),
		SenderEndpointRef: senderEndpointRef(metadata),
		TargetCapability:  capability,
		CorrelationRef:    correlationRef(req.Message),
		ReadbackRequired:  metadataBool(metadata, "readback_required", "tc_readback_required"),
		QualityGate:       qualityGate,
		Payload: contracts.Payload{
			Summary:    summaryFromMetadataOrBody(metadata, body),
			Body:       body,
			References: references,
		},
		Constraints: constraintsFromMetadata(metadata),
	}, nil
}

func SendMessageResponseFromIngress(req SendMessageRequest, accepted contracts.MessageIngressResponse) SendMessageResponse {
	task := Task{
		ID:        accepted.MessageRef,
		ContextID: firstNonEmpty(req.Message.ContextID, req.Message.TaskID, accepted.MessageRef),
		Status: TaskStatus{
			State: TaskStateSubmitted,
			Message: &Message{
				MessageID: "tc://a2a-message/accepted-" + compactRef(accepted.MessageRef),
				ContextID: firstNonEmpty(req.Message.ContextID, req.Message.TaskID, accepted.MessageRef),
				TaskID:    accepted.MessageRef,
				Role:      RoleAgent,
				Parts: []Part{{
					Text:      "touch-connect accepted the A2A message",
					MediaType: mediaTypeTextPlain,
				}},
			},
		},
		History: []Message{req.Message},
		Metadata: map[string]any{
			"tc_message_ref":          accepted.MessageRef,
			"tc_delivery_ref":         accepted.DeliveryRef,
			"tc_quality_decision_ref": accepted.QualityDecisionRef,
			"tc_state":                accepted.State,
		},
	}
	return SendMessageResponse{Task: &task}
}

func TaskFromSnapshot(taskID string, snapshot contracts.SnapshotResponse) (Task, bool) {
	message, ok := findTaskMessage(taskID, snapshot.Messages)
	if !ok {
		return Task{}, false
	}
	return Task{
		ID:        message.MessageRef,
		ContextID: firstNonEmpty(message.CorrelationRef, message.MessageRef),
		Status: TaskStatus{
			State: taskStateFromMessageState(message.State),
			Message: &Message{
				MessageID: "tc://a2a-message/status-" + compactRef(message.MessageRef),
				ContextID: firstNonEmpty(message.CorrelationRef, message.MessageRef),
				TaskID:    message.MessageRef,
				Role:      RoleAgent,
				Parts: []Part{{
					Text:      message.Payload.Summary,
					MediaType: mediaTypeTextPlain,
				}},
			},
		},
		Artifacts: artifactsFromSnapshot(message, snapshot.Artifacts),
		History:   []Message{messageFromRecord(message)},
		Metadata: map[string]any{
			"tc_message_ref":        message.MessageRef,
			"tc_delivery_ref":       message.DeliveryRef,
			"tc_correlation_ref":    message.CorrelationRef,
			"tc_target_capability":  message.TargetCapability,
			"tc_redelivery_count":   message.RedeliveryCount,
			"tc_readback_required":  message.ReadbackRequired,
			"tc_latest_attempt_ref": message.AttemptRef,
		},
	}, true
}

func AgentCardFromSnapshot(baseURL string, snapshot contracts.SnapshotResponse, version string) AgentCard {
	return AgentCard{
		Name:        "touch-connect",
		Description: "Message-quality and handoff-governance layer for heterogeneous AI agents.",
		SupportedInterfaces: []AgentInterface{{
			URL:             strings.TrimRight(baseURL, "/") + "/a2a/rpc",
			ProtocolBinding: "JSONRPC",
			ProtocolVersion: ProtocolVersion,
		}},
		Version: version,
		Capabilities: AgentCapabilities{
			Streaming:         false,
			PushNotifications: false,
		},
		DefaultInputModes:  []string{mediaTypeTextPlain, mediaTypeApplicationJSON},
		DefaultOutputModes: []string{mediaTypeApplicationJSON, mediaTypeTextPlain},
		Skills:             skillsFromSnapshot(snapshot),
	}
}

func ErrorResponse(id any, code int, message string, data any) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func ResultResponse(id any, result any) JSONRPCResponse {
	return JSONRPCResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func ErrorCode(err error) int {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return ErrorInvalidRequest
	case errors.Is(err, ErrInvalidParams), errors.Is(err, ErrCapabilityRequired), errors.Is(err, ErrMessagePartRequired):
		return ErrorInvalidParams
	case errors.Is(err, ErrTaskNotFound):
		return ErrorTaskNotFound
	case errors.Is(err, ErrVersionNotSupported):
		return ErrorVersionUnsupported
	case errors.Is(err, ErrUnsupportedA2AMethod):
		return ErrorMethodNotFound
	default:
		return ErrorInternal
	}
}

func mergeMetadata(items ...map[string]any) map[string]any {
	merged := map[string]any{}
	for _, item := range items {
		for key, value := range item {
			merged[key] = value
		}
	}
	return merged
}

func partsToPayload(message Message) (string, []contracts.Reference, error) {
	segments := make([]string, 0)
	references := make([]contracts.Reference, 0)
	for _, part := range message.Parts {
		switch {
		case strings.TrimSpace(part.Text) != "":
			segments = append(segments, part.Text)
		case part.Data != nil:
			raw, err := json.Marshal(part.Data)
			if err != nil {
				return "", nil, err
			}
			segments = append(segments, string(raw))
		case strings.TrimSpace(part.URL) != "":
			references = append(references, contracts.Reference{
				Ref:   part.URL,
				Type:  "a2a_url",
				Title: firstNonEmpty(part.Filename, part.URL),
			})
			segments = append(segments, part.URL)
		}
	}
	for _, ref := range message.ReferenceTaskIDs {
		if strings.TrimSpace(ref) == "" {
			continue
		}
		references = append(references, contracts.Reference{Ref: ref, Type: "a2a_task"})
	}
	body := strings.TrimSpace(strings.Join(segments, "\n\n"))
	if body == "" {
		return "", nil, ErrMessagePartRequired
	}
	return body, references, nil
}

func metadataString(metadata map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if strings.TrimSpace(typed.String()) != "" {
				return strings.TrimSpace(typed.String())
			}
		}
	}
	return ""
}

func metadataBool(metadata map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return typed == "true"
		}
	}
	return false
}

func qualityGateFromMetadata(metadata map[string]any) (contracts.QualityGateMode, error) {
	value := metadataString(metadata, "quality_gate", "tc_quality_gate")
	if value == "" {
		return contracts.QualityGateEnforce, nil
	}
	return contracts.ParseQualityGateMode(value)
}

func senderEndpointRef(metadata map[string]any) string {
	return firstNonEmpty(metadataString(metadata, "sender_endpoint_ref", "tc_sender_endpoint_ref"), "tc://endpoint/a2a-client")
}

func correlationRef(message Message) string {
	return firstNonEmpty(message.TaskID, message.ContextID)
}

func summaryFromMetadataOrBody(metadata map[string]any, body string) string {
	if summary := metadataString(metadata, "summary", "tc_summary"); summary != "" {
		return summary
	}
	line := strings.TrimSpace(strings.Split(body, "\n")[0])
	if len(line) > 120 {
		return line[:120]
	}
	return line
}

func constraintsFromMetadata(metadata map[string]any) []contracts.Constraint {
	constraint := metadataString(metadata, "constraint", "tc_constraint")
	if constraint == "" {
		return []contracts.Constraint{}
	}
	return []contracts.Constraint{{Code: "a2a_constraint", Summary: constraint}}
}

func findTaskMessage(taskID string, messages []contracts.MessageRecord) (contracts.MessageRecord, bool) {
	for _, message := range messages {
		if message.MessageRef == taskID || message.CorrelationRef == taskID {
			return message, true
		}
	}
	return contracts.MessageRecord{}, false
}

func messageFromRecord(message contracts.MessageRecord) Message {
	return Message{
		MessageID: message.MessageRef,
		ContextID: firstNonEmpty(message.CorrelationRef, message.MessageRef),
		TaskID:    message.MessageRef,
		Role:      RoleUser,
		Parts: []Part{{
			Text:      message.Payload.Body,
			MediaType: mediaTypeTextPlain,
		}},
		Metadata: map[string]any{
			"tc_message_ref":       message.MessageRef,
			"tc_target_capability": message.TargetCapability,
		},
	}
}

func artifactsFromSnapshot(message contracts.MessageRecord, artifacts []contracts.ArtifactRecord) []Artifact {
	result := make([]Artifact, 0)
	for _, artifact := range artifacts {
		if artifact.MessageRef != message.MessageRef && artifact.TaskRef != message.CorrelationRef {
			continue
		}
		result = append(result, Artifact{
			ArtifactID:  artifact.ArtifactVersionRef,
			Name:        artifact.Kind,
			Description: artifact.StorageRef,
			Parts: []Part{{
				Text:      artifact.StorageRef,
				MediaType: mediaTypeTextPlain,
				Metadata: map[string]any{
					"tc_media_type": artifact.MediaType,
					"tc_checksum":   artifact.Checksum,
				},
			}},
			Metadata: map[string]any{
				"tc_artifact_ref":         artifact.ArtifactRef,
				"tc_artifact_version_ref": artifact.ArtifactVersionRef,
				"tc_message_ref":          artifact.MessageRef,
				"tc_attempt_ref":          artifact.AttemptRef,
				"tc_task_ref":             artifact.TaskRef,
			},
		})
	}
	return result
}

func skillsFromSnapshot(snapshot contracts.SnapshotResponse) []AgentSkill {
	seen := map[string]bool{}
	skills := make([]AgentSkill, 0)
	for _, endpoint := range snapshot.Endpoints {
		for name := range endpoint.Capabilities {
			if seen[name] {
				continue
			}
			seen[name] = true
			skills = append(skills, AgentSkill{
				ID:          name,
				Name:        name,
				Description: "touch-connect capability " + name,
				Tags:        []string{"touch-connect", "handoff", name},
				InputModes:  []string{mediaTypeTextPlain, mediaTypeApplicationJSON},
				OutputModes: []string{mediaTypeApplicationJSON, mediaTypeTextPlain},
			})
		}
	}
	return skills
}

func taskStateFromMessageState(state string) string {
	switch state {
	case "available":
		return TaskStateSubmitted
	case "claimed", "processing", "takeover_candidate":
		return TaskStateWorking
	case "completed":
		return TaskStateCompleted
	case "failed", "dead_lettered":
		return TaskStateFailed
	case "canceled":
		return TaskStateCanceled
	case "input_required":
		return TaskStateInputRequired
	default:
		return TaskStateUnknown
	}
}

func validClientRole(role string) bool {
	switch role {
	case RoleUser, "user", "USER":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func compactRef(ref string) string {
	ref = strings.TrimPrefix(ref, "tc://")
	return strings.NewReplacer("/", "_", ":", "_").Replace(ref)
}
