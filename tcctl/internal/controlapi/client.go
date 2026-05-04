package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *Client) Health(ctx context.Context) (contracts.HealthResponse, error) {
	var out contracts.HealthResponse
	err := c.get(ctx, "/healthz", &out)
	return out, err
}

func (c *Client) Version(ctx context.Context) (contracts.VersionResponse, error) {
	var out contracts.VersionResponse
	err := c.get(ctx, "/version", &out)
	return out, err
}

func (c *Client) Snapshot(ctx context.Context) (contracts.SnapshotResponse, error) {
	var out contracts.SnapshotResponse
	err := c.get(ctx, "/v1/snapshot", &out)
	return out, err
}

func (c *Client) Endpoints(ctx context.Context) ([]contracts.EndpointRecord, error) {
	var out []contracts.EndpointRecord
	err := c.get(ctx, "/v1/endpoints", &out)
	return out, err
}

func (c *Client) Endpoint(ctx context.Context, ref string) (contracts.EndpointRecord, error) {
	var out contracts.EndpointRecord
	err := c.get(ctx, "/v1/endpoints/inspect?ref="+url.QueryEscape(ref), &out)
	return out, err
}

func (c *Client) Capabilities(ctx context.Context) (map[string][]string, error) {
	var out map[string][]string
	err := c.get(ctx, "/v1/capabilities", &out)
	return out, err
}

func (c *Client) SendMessage(ctx context.Context, req contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	var out contracts.MessageIngressResponse
	err := c.post(ctx, "/v1/messages", req, &out)
	return out, err
}

func (c *Client) Messages(ctx context.Context, taskRef string) ([]contracts.MessageRecord, error) {
	var out []contracts.MessageRecord
	path := "/v1/messages"
	if taskRef != "" {
		path += "?task=" + url.QueryEscape(taskRef)
	}
	err := c.get(ctx, path, &out)
	return out, err
}

func (c *Client) Message(ctx context.Context, ref string) (contracts.MessageRecord, error) {
	var out contracts.MessageRecord
	err := c.get(ctx, "/v1/messages/inspect?ref="+url.QueryEscape(ref), &out)
	return out, err
}

func (c *Client) TaskStatus(ctx context.Context, taskRef string) (map[string]any, error) {
	var out map[string]any
	err := c.get(ctx, "/v1/tasks/status?task="+url.QueryEscape(taskRef), &out)
	return out, err
}

func (c *Client) TaskHistory(ctx context.Context, taskRef string) (map[string]any, error) {
	var out map[string]any
	err := c.get(ctx, "/v1/tasks/history?task="+url.QueryEscape(taskRef), &out)
	return out, err
}

func (c *Client) CancelTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	var out contracts.TaskCommandResponse
	err := c.post(ctx, "/v1/tasks/cancel", req, &out)
	return out, err
}

func (c *Client) RetryTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	var out contracts.TaskCommandResponse
	err := c.post(ctx, "/v1/tasks/retry", req, &out)
	return out, err
}

func (c *Client) Artifacts(ctx context.Context, taskRef string) ([]contracts.ArtifactRecord, error) {
	var out []contracts.ArtifactRecord
	path := "/v1/artifacts"
	if taskRef != "" {
		path += "?task=" + url.QueryEscape(taskRef)
	}
	err := c.get(ctx, path, &out)
	return out, err
}

func (c *Client) Artifact(ctx context.Context, ref string) (contracts.ArtifactRecord, error) {
	var out contracts.ArtifactRecord
	err := c.get(ctx, "/v1/artifacts/inspect?ref="+url.QueryEscape(ref), &out)
	return out, err
}

func (c *Client) FinalizeArtifact(ctx context.Context, req contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	var out contracts.ArtifactFinalizeResponse
	err := c.post(ctx, "/v1/artifacts/finalize", req, &out)
	return out, err
}

func (c *Client) Approvals(ctx context.Context) ([]contracts.ApprovalRecord, error) {
	var out []contracts.ApprovalRecord
	err := c.get(ctx, "/v1/approvals", &out)
	return out, err
}

func (c *Client) Approval(ctx context.Context, ref string) (contracts.ApprovalRecord, error) {
	var out contracts.ApprovalRecord
	err := c.get(ctx, "/v1/approvals/inspect?ref="+url.QueryEscape(ref), &out)
	return out, err
}

func (c *Client) RecordApproval(ctx context.Context, req contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	var out contracts.ApprovalDecisionResponse
	err := c.post(ctx, "/v1/approvals/decide", req, &out)
	return out, err
}

func (c *Client) DeadLetters(ctx context.Context) ([]contracts.DeadLetterRecord, error) {
	var out []contracts.DeadLetterRecord
	err := c.get(ctx, "/v1/dlq", &out)
	return out, err
}

func (c *Client) DeadLetter(ctx context.Context, ref string) (contracts.DeadLetterRecord, error) {
	var out contracts.DeadLetterRecord
	err := c.get(ctx, "/v1/dlq/inspect?ref="+url.QueryEscape(ref), &out)
	return out, err
}

func (c *Client) ReplayDeadLetter(ctx context.Context, req contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	var out contracts.DLQReplayResponse
	err := c.post(ctx, "/v1/dlq/replay", req, &out)
	return out, err
}

func (c *Client) SideEffects(ctx context.Context, taskRef string) ([]contracts.SideEffectRecord, error) {
	var out []contracts.SideEffectRecord
	path := "/v1/side-effects"
	if taskRef != "" {
		path += "?task=" + url.QueryEscape(taskRef)
	}
	err := c.get(ctx, path, &out)
	return out, err
}

func (c *Client) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, target)
}

func (c *Client) post(ctx context.Context, path string, value any, target any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, target)
}

func (c *Client) do(req *http.Request, target any) error {
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= http.StatusBadRequest {
		var apiErr contracts.ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&apiErr); err == nil && apiErr.Code != "" {
			return fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
		}
		return fmt.Errorf("control_status_%d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
