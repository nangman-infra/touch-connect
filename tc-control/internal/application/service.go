package application

import (
	"context"
	"errors"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type ServerClient interface {
	Health(context.Context) (contracts.HealthResponse, error)
	Version(context.Context) (contracts.VersionResponse, error)
	Snapshot(context.Context) (contracts.SnapshotResponse, error)
	SendMessage(context.Context, contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error)
	RecordApproval(context.Context, contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error)
	CancelTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error)
	RetryTask(context.Context, contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error)
	ReplayDeadLetter(context.Context, contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error)
	FinalizeArtifact(context.Context, contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error)
}

type Service struct {
	server  ServerClient
	version string
}

func NewService(server ServerClient, version string) (*Service, error) {
	if server == nil {
		return nil, errors.New("server client is required")
	}
	if version == "" {
		version = "0.1.0-dev"
	}
	return &Service{server: server, version: version}, nil
}

func (s *Service) Health() contracts.HealthResponse {
	return contracts.HealthResponse{Status: "ok", Component: "tc-control", Version: s.version}
}

func (s *Service) Ready(ctx context.Context) (contracts.HealthResponse, error) {
	serverHealth, err := s.server.Health(ctx)
	if err != nil {
		return contracts.HealthResponse{}, err
	}
	return contracts.HealthResponse{
		Status:    "ready",
		Component: "tc-control",
		Version:   serverHealth.Version,
	}, nil
}

func (s *Service) Version(ctx context.Context) (contracts.VersionResponse, error) {
	serverVersion, err := s.server.Version(ctx)
	if err != nil {
		return contracts.VersionResponse{}, err
	}
	return contracts.VersionResponse{
		Version:         s.version,
		MinimumWorker:   serverVersion.MinimumWorker,
		ContractVersion: serverVersion.ContractVersion,
	}, nil
}

func (s *Service) Snapshot(ctx context.Context) (contracts.SnapshotResponse, error) {
	return s.server.Snapshot(ctx)
}

func (s *Service) SendMessage(ctx context.Context, req contracts.MessageIngressRequest) (contracts.MessageIngressResponse, error) {
	return s.server.SendMessage(ctx, req)
}

func (s *Service) RecordApproval(ctx context.Context, req contracts.ApprovalCommandRequest) (contracts.ApprovalDecisionResponse, error) {
	return s.server.RecordApproval(ctx, req)
}

func (s *Service) CancelTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return s.server.CancelTask(ctx, req)
}

func (s *Service) RetryTask(ctx context.Context, req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	return s.server.RetryTask(ctx, req)
}

func (s *Service) ReplayDeadLetter(ctx context.Context, req contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	return s.server.ReplayDeadLetter(ctx, req)
}

func (s *Service) FinalizeArtifact(ctx context.Context, req contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	return s.server.FinalizeArtifact(ctx, req)
}

func (s *Service) Endpoints(ctx context.Context) ([]contracts.EndpointRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	return snapshot.Endpoints, err
}

func (s *Service) Endpoint(ctx context.Context, ref string) (contracts.EndpointRecord, bool, error) {
	items, err := s.Endpoints(ctx)
	if err != nil {
		return contracts.EndpointRecord{}, false, err
	}
	for _, item := range items {
		if item.EndpointRef == ref {
			return item, true, nil
		}
	}
	return contracts.EndpointRecord{}, false, nil
}

func (s *Service) Messages(ctx context.Context, taskRef string) ([]contracts.MessageRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if taskRef == "" {
		return snapshot.Messages, nil
	}
	items := make([]contracts.MessageRecord, 0)
	for _, item := range snapshot.Messages {
		if item.CorrelationRef == taskRef {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Service) Message(ctx context.Context, ref string) (contracts.MessageRecord, bool, error) {
	items, err := s.Messages(ctx, "")
	if err != nil {
		return contracts.MessageRecord{}, false, err
	}
	for _, item := range items {
		if item.MessageRef == ref {
			return item, true, nil
		}
	}
	return contracts.MessageRecord{}, false, nil
}

func (s *Service) Artifacts(ctx context.Context, taskRef string) ([]contracts.ArtifactRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if taskRef == "" {
		return snapshot.Artifacts, nil
	}
	items := make([]contracts.ArtifactRecord, 0)
	for _, item := range snapshot.Artifacts {
		if item.TaskRef == taskRef {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Service) Artifact(ctx context.Context, ref string) (contracts.ArtifactRecord, bool, error) {
	items, err := s.Artifacts(ctx, "")
	if err != nil {
		return contracts.ArtifactRecord{}, false, err
	}
	for _, item := range items {
		if item.ArtifactVersionRef == ref {
			return item, true, nil
		}
	}
	return contracts.ArtifactRecord{}, false, nil
}
