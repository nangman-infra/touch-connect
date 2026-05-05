package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	LLMProviderOpenAIResponses = "openai_responses"
	defaultOpenAIResponsesURL  = "https://api.openai.com/v1"
	defaultLLMTimeout          = 60 * time.Second
)

type LLMExecutorOptions struct {
	Provider        string
	BaseURL         string
	APIKey          string
	Model           string
	SystemPrompt    string
	Timeout         time.Duration
	MaxOutputTokens int
	HTTPClient      *http.Client
}

type LLMExecutor struct {
	provider        string
	baseURL         string
	apiKey          string
	model           string
	systemPrompt    string
	timeout         time.Duration
	maxOutputTokens int
	httpClient      *http.Client
}

type openAIResponsesRequest struct {
	Model           string `json:"model"`
	Instructions    string `json:"instructions,omitempty"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens,omitempty"`
	Store           bool   `json:"store"`
}

type openAIResponsesResponse struct {
	ID     string                `json:"id"`
	Status string                `json:"status"`
	Output []openAIResponsesItem `json:"output"`
	Error  *openAIResponseError  `json:"error,omitempty"`
}

type openAIResponsesItem struct {
	Type    string                   `json:"type"`
	Status  string                   `json:"status,omitempty"`
	Role    string                   `json:"role,omitempty"`
	Content []openAIResponsesContent `json:"content,omitempty"`
}

type openAIResponsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewLLMExecutor(options LLMExecutorOptions) (*LLMExecutor, error) {
	accepted, err := options.validated()
	if err != nil {
		return nil, err
	}
	httpClient := accepted.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &LLMExecutor{
		provider:        accepted.Provider,
		baseURL:         accepted.BaseURL,
		apiKey:          accepted.APIKey,
		model:           accepted.Model,
		systemPrompt:    accepted.SystemPrompt,
		timeout:         accepted.Timeout,
		maxOutputTokens: accepted.MaxOutputTokens,
		httpClient:      httpClient,
	}, nil
}

func (e *LLMExecutor) Execute(ctx context.Context, input ExecutionInput) (ExecutionResult, error) {
	switch e.provider {
	case LLMProviderOpenAIResponses:
		return e.executeOpenAIResponses(ctx, input), nil
	default:
		return ExecutionResult{}, fmt.Errorf("unsupported LLM provider %q", e.provider)
	}
}

func (e *LLMExecutor) executeOpenAIResponses(ctx context.Context, input ExecutionInput) ExecutionResult {
	startedAt := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	body, err := json.Marshal(openAIResponsesRequest{
		Model:           e.model,
		Instructions:    e.systemPrompt,
		Input:           llmPromptFromInput(input),
		MaxOutputTokens: e.maxOutputTokens,
		Store:           false,
	})
	if err != nil {
		return failedLLMResult("llm_request_encode_failed", err.Error(), "", time.Since(startedAt).Milliseconds())
	}
	req, err := http.NewRequestWithContext(runCtx, http.MethodPost, strings.TrimRight(e.baseURL, "/")+"/responses", bytes.NewReader(body))
	if err != nil {
		return failedLLMResult("llm_request_create_failed", err.Error(), "", time.Since(startedAt).Milliseconds())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	res, err := e.httpClient.Do(req)
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return failedLLMResult("llm_timeout", "LLM provider request timed out", "", time.Since(startedAt).Milliseconds())
		}
		return failedLLMResult("llm_request_failed", err.Error(), "", time.Since(startedAt).Milliseconds())
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return failedLLMResult("llm_response_read_failed", err.Error(), "", time.Since(startedAt).Milliseconds())
	}
	durationMS := time.Since(startedAt).Milliseconds()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return failedLLMResult("llm_provider_status", fmt.Sprintf("LLM provider returned status %d", res.StatusCode), string(responseBody), durationMS)
	}
	var providerResponse openAIResponsesResponse
	if err := json.Unmarshal(responseBody, &providerResponse); err != nil {
		return failedLLMResult("llm_response_decode_failed", err.Error(), string(responseBody), durationMS)
	}
	if providerResponse.Error != nil {
		message := providerResponse.Error.Message
		if message == "" {
			message = providerResponse.Error.Code
		}
		return failedLLMResult("llm_provider_error", message, string(responseBody), durationMS)
	}
	output := strings.TrimSpace(outputTextFromOpenAIResponse(providerResponse))
	if output == "" {
		return failedLLMResult("llm_empty_output", "LLM provider returned no output text", string(responseBody), durationMS)
	}
	return ExecutionResult{
		Outcome:    ExecutionOutcomeCompleted,
		Summary:    output,
		Stdout:     output,
		ExitCode:   0,
		DurationMS: durationMS,
	}
}

func (o LLMExecutorOptions) validated() (LLMExecutorOptions, error) {
	if o.Provider == "" {
		o.Provider = LLMProviderOpenAIResponses
	}
	if o.Provider != LLMProviderOpenAIResponses {
		return LLMExecutorOptions{}, fmt.Errorf("unsupported LLM provider %q", o.Provider)
	}
	if o.BaseURL == "" {
		o.BaseURL = defaultOpenAIResponsesURL
	}
	parsed, err := url.Parse(o.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return LLMExecutorOptions{}, errors.New("LLM base URL must be absolute")
	}
	if strings.TrimSpace(o.APIKey) == "" {
		return LLMExecutorOptions{}, errors.New("LLM API key is required")
	}
	o.APIKey = strings.TrimSpace(o.APIKey)
	if strings.TrimSpace(o.Model) == "" {
		return LLMExecutorOptions{}, errors.New("LLM model is required")
	}
	o.Model = strings.TrimSpace(o.Model)
	if o.Timeout == 0 {
		o.Timeout = defaultLLMTimeout
	}
	if o.Timeout < 0 {
		return LLMExecutorOptions{}, errors.New("LLM timeout must not be negative")
	}
	if o.MaxOutputTokens < 0 {
		return LLMExecutorOptions{}, errors.New("LLM max output tokens must not be negative")
	}
	return o, nil
}

func llmPromptFromInput(input ExecutionInput) string {
	var builder strings.Builder
	builder.WriteString("You are executing a touch-connect handoff as an AI worker.\n")
	builder.WriteString("Return a concise completion summary that another agent or operator can audit.\n\n")
	writeLLMPromptLine(&builder, "message_ref", input.MessageRef)
	writeLLMPromptLine(&builder, "attempt_ref", input.AttemptRef)
	writeLLMPromptLine(&builder, "target_capability", input.TargetCapability)
	writeLLMPromptLine(&builder, "correlation_ref", input.CorrelationRef)
	if input.Takeover {
		builder.WriteString("takeover: true\n")
		writeLLMPromptLine(&builder, "last_checkpoint_ref", input.LastCheckpointRef)
		writeLLMPromptLine(&builder, "resume_summary", input.ResumeSummary)
	}
	writeLLMHandoffContext(&builder, input.HandoffContext)
	builder.WriteString("\npayload.summary:\n")
	builder.WriteString(input.Payload.Summary)
	builder.WriteString("\n\npayload.body:\n")
	builder.WriteString(input.Payload.Body)
	if len(input.Constraints) > 0 {
		builder.WriteString("\n\nconstraints:\n")
		for _, constraint := range input.Constraints {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(constraint.Code))
			if strings.TrimSpace(constraint.Summary) != "" {
				builder.WriteString(": ")
				builder.WriteString(strings.TrimSpace(constraint.Summary))
			}
			if strings.TrimSpace(constraint.Details) != "" {
				builder.WriteString(" (")
				builder.WriteString(strings.TrimSpace(constraint.Details))
				builder.WriteString(")")
			}
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func writeLLMHandoffContext(builder *strings.Builder, context HandoffContext) {
	if context.Empty() {
		return
	}
	builder.WriteString("\nhandoff_context:\n")
	writeLLMPromptLine(builder, "task_ref", context.TaskRef)
	if len(context.Messages) > 0 {
		builder.WriteString("prior_messages:\n")
		for _, message := range context.Messages {
			builder.WriteString("- message_ref: ")
			builder.WriteString(message.MessageRef)
			builder.WriteByte('\n')
			writeIndentedPromptLine(builder, "target_capability", message.TargetCapability)
			writeIndentedPromptLine(builder, "state", message.State)
			writeIndentedPromptLine(builder, "attempt_ref", message.AttemptRef)
			if message.RedeliveryCount > 0 {
				writeIndentedPromptLine(builder, "redelivery_count", strconv.Itoa(message.RedeliveryCount))
			}
			writeIndentedPromptBlock(builder, "summary", message.Summary)
			writeIndentedPromptBlock(builder, "body", message.Body)
		}
	}
	if len(context.Artifacts) > 0 {
		builder.WriteString("prior_artifacts:\n")
		for _, artifact := range context.Artifacts {
			builder.WriteString("- artifact_version_ref: ")
			builder.WriteString(artifact.ArtifactVersionRef)
			builder.WriteByte('\n')
			writeIndentedPromptLine(builder, "artifact_ref", artifact.ArtifactRef)
			writeIndentedPromptLine(builder, "message_ref", artifact.MessageRef)
			writeIndentedPromptLine(builder, "attempt_ref", artifact.AttemptRef)
			writeIndentedPromptLine(builder, "kind", artifact.Kind)
			writeIndentedPromptLine(builder, "media_type", artifact.MediaType)
			writeIndentedPromptLine(builder, "storage_ref", artifact.StorageRef)
			if len(artifact.UsedSkillRefs) > 0 {
				writeIndentedPromptLine(builder, "used_skill_refs", strings.Join(artifact.UsedSkillRefs, ", "))
			}
			writeIndentedPromptBlock(builder, "summary", artifact.Summary)
			writeIndentedPromptBlock(builder, "stdout", artifact.Stdout)
			writeIndentedPromptBlock(builder, "stderr", artifact.Stderr)
			writeIndentedPromptBlock(builder, "content", artifact.Content)
		}
	}
}

func writeLLMPromptLine(builder *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func writeIndentedPromptLine(builder *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString("  ")
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func writeIndentedPromptBlock(builder *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString("  ")
	builder.WriteString(key)
	builder.WriteString(":\n")
	for _, line := range strings.Split(value, "\n") {
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
}

func outputTextFromOpenAIResponse(response openAIResponsesResponse) string {
	var parts []string
	for _, item := range response.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, strings.TrimSpace(content.Text))
			}
		}
	}
	return strings.Join(parts, "\n")
}

func failedLLMResult(reason string, summary string, stderr string, durationMS int64) ExecutionResult {
	return ExecutionResult{
		Outcome:           ExecutionOutcomeFailed,
		Summary:           summary,
		FailureReasonCode: reason,
		Stderr:            stderr,
		ExitCode:          -1,
		DurationMS:        durationMS,
	}
}
