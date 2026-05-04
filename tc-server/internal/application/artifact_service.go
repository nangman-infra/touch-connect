package application

import (
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Service) RegisterArtifactVersion(attemptRef string, req contracts.ArtifactVersionRequest) (contracts.ArtifactVersionResponse, error) {
	if err := domain.ValidateArtifactVersion(req); err != nil {
		return contracts.ArtifactVersionResponse{}, err
	}
	attempt, err := s.requireLiveAttempt(attemptRef, req.EndpointRef)
	if err != nil {
		return contracts.ArtifactVersionResponse{}, err
	}
	endpoint, ok := s.endpoints.GetEndpoint(req.EndpointRef)
	if !ok {
		return contracts.ArtifactVersionResponse{}, domain.ErrEndpointNotFound
	}
	if _, ok := s.messages.GetMessage(attempt.MessageRef); !ok {
		return contracts.ArtifactVersionResponse{}, domain.ErrMessageNotFound
	}
	for _, ref := range req.BasedOnArtifactVersionRefs {
		if _, ok := s.store.GetArtifactVersion(ref); !ok {
			return contracts.ArtifactVersionResponse{}, domain.ErrArtifactNotFound
		}
	}
	version := domain.ArtifactVersion{
		ArtifactRef:                req.ArtifactRef,
		ArtifactVersionRef:         req.ArtifactVersionRef,
		RoomRef:                    req.RoomRef,
		TaskRef:                    req.TaskRef,
		TaskRevision:               req.TaskRevision,
		Kind:                       req.Kind,
		MediaType:                  req.MediaType,
		SizeBytes:                  req.SizeBytes,
		Checksum:                   req.Checksum,
		StorageRef:                 req.StorageRef,
		RetentionClass:             req.RetentionClass,
		AccessScope:                req.AccessScope,
		BasedOnMessageRefs:         req.BasedOnMessageRefs,
		BasedOnArtifactVersionRefs: req.BasedOnArtifactVersionRefs,
		CreatedByActorID:           endpoint.ActorID,
		CreatedByEndpointRef:       endpoint.EndpointRef,
		MessageRef:                 attempt.MessageRef,
		AttemptRef:                 attempt.AttemptRef,
		CreatedAt:                  s.now(),
	}
	if err := s.store.SaveArtifactVersion(version); err != nil {
		return contracts.ArtifactVersionResponse{}, err
	}
	return contracts.ArtifactVersionResponse{
		ArtifactRef:          version.ArtifactRef,
		ArtifactVersionRef:   version.ArtifactVersionRef,
		AttemptRef:           version.AttemptRef,
		MessageRef:           version.MessageRef,
		CreatedByEndpointRef: version.CreatedByEndpointRef,
		CreatedAt:            formatTime(version.CreatedAt),
	}, nil
}

func (s *Service) validateCheckpointArtifactRefs(attempt domain.Attempt, artifactRefs []string) error {
	for _, ref := range artifactRefs {
		version, ok := s.store.GetArtifactVersion(ref)
		if !ok {
			return domain.ErrArtifactNotFound
		}
		if version.MessageRef != attempt.MessageRef {
			return domain.ErrArtifactNotFound
		}
	}
	return nil
}

func (s *Service) requireLiveAttempt(attemptRef string, endpointRef string) (domain.Attempt, error) {
	attempt, ok := s.processing.GetAttempt(attemptRef)
	if !ok {
		return domain.Attempt{}, domain.ErrAttemptNotFound
	}
	if attempt.EndpointRef != endpointRef || attemptClosed(attempt.State) {
		return domain.Attempt{}, domain.ErrStaleAttempt
	}
	if s.leaseExpired(attempt) {
		return domain.Attempt{}, domain.ErrLeaseExpired
	}
	return attempt, nil
}
