package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  httpClient,
	}
}

func (c *HTTPClient) Health(ctx context.Context) (contracts.HealthResponse, error) {
	var res contracts.HealthResponse
	err := c.get(ctx, "/healthz", &res)
	return res, err
}

func (c *HTTPClient) Version(ctx context.Context) (contracts.VersionResponse, error) {
	var res contracts.VersionResponse
	err := c.get(ctx, "/version", &res)
	return res, err
}

func (c *HTTPClient) Snapshot(ctx context.Context) (contracts.SnapshotResponse, error) {
	var res contracts.SnapshotResponse
	err := c.get(ctx, "/v1/control/snapshot", &res)
	return res, err
}

func (c *HTTPClient) RegisterEndpoint(ctx context.Context, req contracts.EndpointRegistrationRequest) (contracts.EndpointRegistrationResponse, error) {
	var res contracts.EndpointRegistrationResponse
	err := c.post(ctx, "/v1/endpoints/register", req, &res)
	return res, err
}

func (c *HTTPClient) HeartbeatEndpoint(ctx context.Context, endpointRef string, req contracts.EndpointHeartbeatRequest) (contracts.EndpointHeartbeatResponse, error) {
	var res contracts.EndpointHeartbeatResponse
	err := c.post(ctx, "/v1/endpoints/"+endpointRef+"/heartbeat", req, &res)
	return res, err
}

func (c *HTTPClient) AdvertiseCapabilities(ctx context.Context, endpointRef string, req contracts.CapabilityAdvertisementRequest) (contracts.CapabilityAdvertisementResponse, error) {
	var res contracts.CapabilityAdvertisementResponse
	err := c.post(ctx, "/v1/endpoints/"+endpointRef+"/capabilities", req, &res)
	return res, err
}

func (c *HTTPClient) ClaimMessage(ctx context.Context, messageRef string, req contracts.ClaimMessageRequest) (contracts.ClaimMessageResponse, error) {
	var res contracts.ClaimMessageResponse
	err := c.post(ctx, "/v1/messages/"+messageRef+"/claim", req, &res)
	return res, err
}

func (c *HTTPClient) ClaimNextMessage(ctx context.Context, req contracts.ClaimNextMessageRequest) (contracts.ClaimNextMessageResponse, error) {
	var res contracts.ClaimNextMessageResponse
	err := c.post(ctx, "/v1/messages/claim-next", req, &res)
	return res, err
}

func (c *HTTPClient) SubmitCheckpoint(ctx context.Context, attemptRef string, req contracts.CheckpointRequest) (contracts.CheckpointResponse, error) {
	var res contracts.CheckpointResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/checkpoints", req, &res)
	return res, err
}

func (c *HTTPClient) SubmitReadback(ctx context.Context, attemptRef string, req contracts.ReadbackRequest) (contracts.ReadbackResponse, error) {
	var res contracts.ReadbackResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/readback", req, &res)
	return res, err
}

func (c *HTTPClient) RefreshLease(ctx context.Context, attemptRef string, req contracts.RefreshLeaseRequest) (contracts.RefreshLeaseResponse, error) {
	var res contracts.RefreshLeaseResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/lease", req, &res)
	return res, err
}

func (c *HTTPClient) RegisterArtifactVersion(ctx context.Context, attemptRef string, req contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error) {
	var res contracts.ArtifactVersionResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/artifacts", req, &res)
	return res, err
}

func (c *HTTPClient) RecordApprovalDecision(ctx context.Context, attemptRef string, req contracts.ApprovalDecisionRequest) (contracts.ApprovalDecisionResponse, error) {
	var res contracts.ApprovalDecisionResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/approvals", req, &res)
	return res, err
}

func (c *HTTPClient) StartSideEffectExecution(ctx context.Context, attemptRef string, req contracts.SideEffectExecutionRequest) (contracts.SideEffectExecutionResponse, error) {
	var res contracts.SideEffectExecutionResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/side-effects", req, &res)
	return res, err
}

func (c *HTTPClient) CompleteSideEffectExecution(ctx context.Context, executionRef string, req contracts.CompleteSideEffectExecutionRequest) (contracts.CompleteSideEffectExecutionResponse, error) {
	var res contracts.CompleteSideEffectExecutionResponse
	err := c.post(ctx, "/v1/side-effects/"+executionRef+"/complete", req, &res)
	return res, err
}

func (c *HTTPClient) CompleteAttempt(ctx context.Context, attemptRef string, req contracts.CompleteAttemptRequest) (contracts.CompleteAttemptResponse, error) {
	var res contracts.CompleteAttemptResponse
	err := c.post(ctx, "/v1/attempts/"+attemptRef+"/complete", req, &res)
	return res, err
}

func (c *HTTPClient) get(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiErr contracts.ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&apiErr); err == nil && apiErr.Code != "" {
			return contracts.APIError{StatusCode: res.StatusCode, Response: apiErr}
		}
		return contracts.APIError{
			StatusCode: res.StatusCode,
			Response: contracts.ErrorResponse{
				Code:    fmt.Sprintf("server_status_%d", res.StatusCode),
				Message: fmt.Sprintf("server returned status %d", res.StatusCode),
			},
		}
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func (c *HTTPClient) post(ctx context.Context, path string, body any, target any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var apiErr contracts.ErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&apiErr); err == nil && apiErr.Code != "" {
			return contracts.APIError{StatusCode: res.StatusCode, Response: apiErr}
		}
		return contracts.APIError{
			StatusCode: res.StatusCode,
			Response: contracts.ErrorResponse{
				Code:    fmt.Sprintf("server_status_%d", res.StatusCode),
				Message: fmt.Sprintf("server returned status %d", res.StatusCode),
			},
		}
	}
	return json.NewDecoder(res.Body).Decode(target)
}
