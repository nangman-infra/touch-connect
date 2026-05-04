package serverapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: client}
}

func (c *Client) Health(ctx context.Context) (contracts.HealthResponse, error) {
	var response contracts.HealthResponse
	err := c.get(ctx, "/healthz", &response)
	return response, err
}

func (c *Client) Version(ctx context.Context) (contracts.VersionResponse, error) {
	var response contracts.VersionResponse
	err := c.get(ctx, "/version", &response)
	return response, err
}

func (c *Client) Snapshot(ctx context.Context) (contracts.SnapshotResponse, error) {
	var response contracts.SnapshotResponse
	err := c.get(ctx, "/v1/control/snapshot", &response)
	return response, err
}

func (c *Client) SendMessage(ctx context.Context, req contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	var response contracts.MessageIngressResponse
	err := c.post(ctx, "/v1/messages", req, &response)
	return response, err
}

func (c *Client) RecordApproval(ctx context.Context, req contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	var response contracts.ApprovalDecisionResponse
	approval := contracts.ApprovalDecisionRequest{
		ApprovalRef:             req.ApprovalRef,
		TargetType:              req.TargetType,
		TargetRef:               req.TargetRef,
		RequestedByActorID:      req.RequestedByActorID,
		ApproverSubjectsOrRoles: req.ApproverSubjectsOrRoles,
		ApprovalScope:           req.ApprovalScope,
		ApprovalHash:            req.ApprovalHash,
		Status:                  req.Status,
		Reason:                  req.Reason,
		ExpiresAt:               req.ExpiresAt,
		DecidedByActorID:        req.DecidedByActorID,
		DecisionNote:            req.DecisionNote,
	}
	err := c.post(ctx, "/v1/attempts/"+req.AttemptRef+"/approvals", approval, &response)
	return response, err
}

func (c *Client) CancelTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	var response contracts.TaskCommandResponse
	err := c.post(ctx, "/v1/control/tasks/cancel", req, &response)
	return response, err
}

func (c *Client) RetryTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	var response contracts.TaskCommandResponse
	err := c.post(ctx, "/v1/control/tasks/retry", req, &response)
	return response, err
}

func (c *Client) ReplayDeadLetter(ctx context.Context, req contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	var response contracts.DLQReplayResponse
	err := c.post(ctx, "/v1/control/dlq/replay", req, &response)
	return response, err
}

func (c *Client) FinalizeArtifact(ctx context.Context, req contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	var response contracts.ArtifactFinalizeResponse
	err := c.post(ctx, "/v1/control/artifacts/finalize", req, &response)
	return response, err
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
		return fmt.Errorf("server_status_%d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
