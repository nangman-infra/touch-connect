package application

import (
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Service) FinalizeArtifact(req contracts.ArtifactFinalizeRequest) (contracts.ArtifactFinalizeResponse, error) {
	if req.ArtifactVersionRef == "" || req.ActorID == "" {
		return contracts.ArtifactFinalizeResponse{}, domain.ErrInvalidInput
	}
	if _, ok := s.store.GetArtifactVersion(req.ArtifactVersionRef); !ok {
		return contracts.ArtifactFinalizeResponse{}, domain.ErrArtifactNotFound
	}
	if existing, ok := s.store.GetArtifactFinalization(req.ArtifactVersionRef); ok {
		return artifactFinalizeResponse(existing, true), nil
	}
	finalization := domain.ArtifactFinalization{
		ArtifactVersionRef: req.ArtifactVersionRef,
		FinalizationRef:    s.store.NextRef("artifact-finalization"),
		FinalizedByActorID: req.ActorID,
		Reason:             req.Reason,
		FinalizedAt:        s.now(),
	}
	if err := s.store.SaveArtifactFinalization(finalization); err != nil {
		return contracts.ArtifactFinalizeResponse{}, err
	}
	return artifactFinalizeResponse(finalization, false), nil
}

func artifactFinalizeResponse(finalization domain.ArtifactFinalization, deduped bool) contracts.ArtifactFinalizeResponse {
	return contracts.ArtifactFinalizeResponse{
		ArtifactVersionRef: finalization.ArtifactVersionRef,
		FinalizationRef:    finalization.FinalizationRef,
		State:              "finalized",
		FinalizedByActorID: finalization.FinalizedByActorID,
		FinalizedAt:        formatOptionalTime(finalization.FinalizedAt),
		Deduped:            deduped,
	}
}
