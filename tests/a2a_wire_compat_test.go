package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	tcworker "github.com/nangman-infra/touch-connect/tc-worker"
)

func TestA2AJSONRPCSendMessageAndGetTask(t *testing.T) {
	server := tcserver.NewInMemoryServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	worker := tcworker.NewHTTPRuntime(httpServer.URL, httpServer.Client(), tcworker.DefaultConfig())
	if err := worker.Register(context.Background()); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	var card struct {
		SupportedInterfaces []struct {
			URL             string `json:"url"`
			ProtocolBinding string `json:"protocolBinding"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"supportedInterfaces"`
		Skills []struct {
			ID string `json:"id"`
		} `json:"skills"`
	}
	getJSON(t, httpServer.URL+"/.well-known/agent.json", httpServer.Client(), http.StatusOK, &card)
	if len(card.SupportedInterfaces) != 1 || !strings.HasSuffix(card.SupportedInterfaces[0].URL, "/a2a/rpc") || card.SupportedInterfaces[0].ProtocolVersion != "1.0" {
		t.Fatalf("expected A2A JSON-RPC interface in agent card, got %+v", card.SupportedInterfaces)
	}
	if len(card.Skills) != 1 || card.Skills[0].ID != "code.change" {
		t.Fatalf("expected code.change skill in agent card, got %+v", card.Skills)
	}

	send := map[string]any{
		"jsonrpc": "2.0",
		"id":      "send-1",
		"method":  "SendMessage",
		"params": map[string]any{
			"message": map[string]any{
				"messageId": "a2a-message-1",
				"contextId": "a2a-context-1",
				"role":      "ROLE_USER",
				"parts": []map[string]any{
					{
						"text":      "Implement the A2A compatibility smoke path.",
						"mediaType": "text/plain",
					},
				},
				"metadata": map[string]any{
					"target_capability": "code.change",
					"summary":           "A2A smoke message",
				},
			},
		},
	}
	var sent a2aRPCResponse
	postA2ARPC(t, httpServer, send, &sent)
	if sent.Error != nil {
		t.Fatalf("expected A2A SendMessage success, got %+v", sent.Error)
	}
	var sendResult a2aSendMessageResult
	if err := json.Unmarshal(sent.Result, &sendResult); err != nil {
		t.Fatalf("decode A2A SendMessage result: %v\n%s", err, string(sent.Result))
	}
	if sendResult.Task.ID == "" || sendResult.Task.Status.State != "TASK_STATE_SUBMITTED" {
		t.Fatalf("expected submitted A2A task, got %+v", sendResult.Task)
	}
	if snapshot := server.Snapshot(); len(snapshot.Messages) != 1 || snapshot.Messages[0].TargetCapability != "code.change" || snapshot.Messages[0].CorrelationRef != "a2a-context-1" {
		t.Fatalf("expected A2A message in touch-connect snapshot, got %+v", snapshot.Messages)
	}

	if _, err := worker.ProcessMessage(context.Background(), sendResult.Task.ID); err != nil {
		t.Fatalf("process A2A message: %v", err)
	}
	getTask := map[string]any{
		"jsonrpc": "2.0",
		"id":      "get-1",
		"method":  "GetTask",
		"params": map[string]any{
			"id": sendResult.Task.ID,
		},
	}
	var fetched a2aRPCResponse
	postA2ARPC(t, httpServer, getTask, &fetched)
	if fetched.Error != nil {
		t.Fatalf("expected A2A GetTask success, got %+v", fetched.Error)
	}
	var taskResult a2aTask
	if err := json.Unmarshal(fetched.Result, &taskResult); err != nil {
		t.Fatalf("decode A2A GetTask result: %v\n%s", err, string(fetched.Result))
	}
	if taskResult.ID != sendResult.Task.ID || taskResult.Status.State != "TASK_STATE_COMPLETED" {
		t.Fatalf("expected completed A2A task, got %+v", taskResult)
	}
}

type a2aRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type a2aSendMessageResult struct {
	Task a2aTask `json:"task"`
}

type a2aTask struct {
	ID     string `json:"id"`
	Status struct {
		State string `json:"state"`
	} `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

func postA2ARPC(t *testing.T, server *httptest.Server, payload map[string]any, target any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal A2A payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/a2a/rpc", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create A2A request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("A2A-Version", "1.0")
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("post A2A request: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected A2A status 200, got %d", res.StatusCode)
	}
	if got := res.Header.Get("A2A-Version"); got != "1.0" {
		t.Fatalf("expected A2A-Version response header, got %q", got)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatalf("decode A2A response: %v", err)
	}
}
