package application

import (
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Service) SnapshotResponse() contracts.SnapshotResponse {
	snapshot := s.Snapshot()
	return contracts.SnapshotResponse{
		Endpoints:        endpointRecords(snapshot.Endpoints),
		Messages:         messageRecords(snapshot.Messages),
		Attempts:         attemptRecords(snapshot.Attempts),
		Checkpoints:      checkpointRecords(snapshot.Checkpoints),
		Readbacks:        readbackRecords(snapshot.Readbacks),
		Artifacts:        artifactRecords(snapshot.Artifacts),
		Finalizations:    artifactFinalizationRecords(snapshot.Finalizations),
		DeadLetters:      deadLetterRecords(snapshot.DeadLetters),
		Approvals:        approvalRecords(snapshot.Approvals),
		SideEffects:      sideEffectRecords(snapshot.SideEffects),
		QualityDecisions: snapshot.QualityDecisions,
		Freshness: contracts.FreshnessRecord{
			GeneratedAt: formatTime(s.now()),
			Source:      "tc-server",
		},
	}
}

func endpointRecords(items []domain.Endpoint) []contracts.EndpointRecord {
	records := make([]contracts.EndpointRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.EndpointRecord{
			EndpointRef:       item.EndpointRef,
			DisplayName:       item.DisplayName,
			ActorID:           item.ActorID,
			WorkspaceID:       item.WorkspaceID,
			ConnectionState:   item.ConnectionState,
			Capabilities:      item.Capabilities,
			ExecutionHints:    item.ExecutionHints,
			WorkerVersion:     item.WorkerVersion,
			StartedAt:         item.StartedAt,
			RegisteredAt:      formatOptionalTime(item.RegisteredAt),
			LastHeartbeatAt:   formatOptionalTime(item.LastHeartbeatAt),
			CurrentAttemptRef: item.CurrentAttemptRef,
			LastActivityAt:    formatOptionalTime(item.LastActivityAt),
			ProgressSummary:   item.ProgressSummary,
		})
	}
	return records
}

func messageRecords(items []domain.Message) []contracts.MessageRecord {
	records := make([]contracts.MessageRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.MessageRecord{
			MessageRef:           item.MessageRef,
			DeliveryRef:          item.DeliveryRef,
			SenderEndpointRef:    item.SenderEndpointRef,
			TargetCapability:     item.TargetCapability,
			TargetEndpointRef:    item.TargetEndpointRef,
			PreferredEndpointRef: item.PreferredEndpointRef,
			DependsOnMessageRefs: append([]string(nil), item.DependsOnMessageRefs...),
			Payload:              item.Payload,
			Constraints:          item.Constraints,
			CorrelationRef:       item.CorrelationRef,
			ReadbackRequired:     item.ReadbackRequired,
			State:                item.State,
			AttemptRef:           item.AttemptRef,
			RedeliveryCount:      item.RedeliveryCount,
		})
	}
	return records
}

func attemptRecords(items []domain.Attempt) []contracts.AttemptRecord {
	records := make([]contracts.AttemptRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.AttemptRecord{
			AttemptRef:     item.AttemptRef,
			MessageRef:     item.MessageRef,
			EndpointRef:    item.EndpointRef,
			State:          item.State,
			LeaseExpiresAt: formatOptionalTime(item.LeaseExpiresAt),
			Revision:       item.Revision,
			AttemptNo:      item.AttemptNo,
			ClaimEpoch:     item.ClaimEpoch,
		})
	}
	return records
}

func checkpointRecords(items []domain.Checkpoint) []contracts.CheckpointRecord {
	records := make([]contracts.CheckpointRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.CheckpointRecord{
			CheckpointRef:     item.CheckpointRef,
			AttemptRef:        item.AttemptRef,
			EndpointRef:       item.EndpointRef,
			State:             item.State,
			Summary:           item.Summary,
			Revision:          item.Revision,
			ArtifactRefs:      item.ArtifactRefs,
			FailureReasonCode: item.FailureReasonCode,
			MissingFields:     item.MissingFields,
			MissingReasons:    item.MissingReasons,
		})
	}
	return records
}

func readbackRecords(items []domain.Readback) []contracts.ReadbackRecord {
	records := make([]contracts.ReadbackRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.ReadbackRecord{
			ReadbackRef:    item.ReadbackRef,
			AttemptRef:     item.AttemptRef,
			EndpointRef:    item.EndpointRef,
			Summary:        item.Summary,
			Understanding:  item.Understanding,
			Questions:      item.Questions,
			MissingFields:  item.MissingFields,
			MissingReasons: item.MissingReasons,
			Revision:       item.Revision,
		})
	}
	return records
}

func artifactRecords(items []domain.ArtifactVersion) []contracts.ArtifactRecord {
	records := make([]contracts.ArtifactRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.ArtifactRecord{
			ArtifactRef:                item.ArtifactRef,
			ArtifactVersionRef:         item.ArtifactVersionRef,
			RoomRef:                    item.RoomRef,
			TaskRef:                    item.TaskRef,
			TaskRevision:               item.TaskRevision,
			Kind:                       item.Kind,
			MediaType:                  item.MediaType,
			SizeBytes:                  item.SizeBytes,
			Checksum:                   item.Checksum,
			StorageRef:                 item.StorageRef,
			RetentionClass:             item.RetentionClass,
			AccessScope:                item.AccessScope,
			BasedOnMessageRefs:         item.BasedOnMessageRefs,
			BasedOnArtifactVersionRefs: item.BasedOnArtifactVersionRefs,
			CreatedByActorID:           item.CreatedByActorID,
			CreatedByEndpointRef:       item.CreatedByEndpointRef,
			MessageRef:                 item.MessageRef,
			AttemptRef:                 item.AttemptRef,
			CreatedAt:                  formatOptionalTime(item.CreatedAt),
		})
	}
	return records
}

func artifactFinalizationRecords(items []domain.ArtifactFinalization) []contracts.ArtifactFinalizationRecord {
	records := make([]contracts.ArtifactFinalizationRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.ArtifactFinalizationRecord{
			ArtifactVersionRef: item.ArtifactVersionRef,
			FinalizationRef:    item.FinalizationRef,
			FinalizedByActorID: item.FinalizedByActorID,
			Reason:             item.Reason,
			FinalizedAt:        formatOptionalTime(item.FinalizedAt),
		})
	}
	return records
}

func deadLetterRecords(items []domain.DeadLetter) []contracts.DeadLetterRecord {
	records := make([]contracts.DeadLetterRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.DeadLetterRecord{
			DeadLetterRef:     item.DeadLetterRef,
			MessageRef:        item.MessageRef,
			LastAttemptRef:    item.LastAttemptRef,
			LastCheckpointRef: item.LastCheckpointRef,
			Reason:            item.Reason,
			RedeliveryCount:   item.RedeliveryCount,
			CreatedAt:         formatOptionalTime(item.CreatedAt),
		})
	}
	return records
}

func approvalRecords(items []domain.ApprovalDecision) []contracts.ApprovalRecord {
	records := make([]contracts.ApprovalRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.ApprovalRecord{
			ApprovalRef:             item.ApprovalRef,
			AttemptRef:              item.AttemptRef,
			MessageRef:              item.MessageRef,
			TargetType:              item.TargetType,
			TargetRef:               item.TargetRef,
			RequestedByActorID:      item.RequestedByActorID,
			ApproverSubjectsOrRoles: item.ApproverSubjectsOrRoles,
			ApprovalScope:           item.ApprovalScope,
			ApprovalHash:            item.ApprovalHash,
			Status:                  item.Status,
			Reason:                  item.Reason,
			DecidedByActorID:        item.DecidedByActorID,
			DecisionNote:            item.DecisionNote,
			RequestedAt:             formatOptionalTime(item.RequestedAt),
			ExpiresAt:               formatOptionalTime(item.ExpiresAt),
			DecidedAt:               formatOptionalTime(item.DecidedAt),
		})
	}
	return records
}

func sideEffectRecords(items []domain.SideEffectExecution) []contracts.SideEffectRecord {
	records := make([]contracts.SideEffectRecord, 0, len(items))
	for _, item := range items {
		records = append(records, contracts.SideEffectRecord{
			SideEffectExecutionRef: item.SideEffectExecutionRef,
			IdempotencyKey:         item.IdempotencyKey,
			ProtectedScope:         item.ProtectedScope,
			ApprovalRef:            item.ApprovalRef,
			ApprovalHash:           item.ApprovalHash,
			MessageRef:             item.MessageRef,
			TaskRef:                item.TaskRef,
			AttemptRef:             item.AttemptRef,
			OperationKind:          item.OperationKind,
			ExternalTarget:         item.ExternalTarget,
			RequestedByActorID:     item.RequestedByActorID,
			ExecutedByActorID:      item.ExecutedByActorID,
			ExecutedByEndpointRef:  item.ExecutedByEndpointRef,
			Status:                 item.Status,
			StartedAt:              formatOptionalTime(item.StartedAt),
			CompletedAt:            formatOptionalTime(item.CompletedAt),
			ResultRef:              item.ResultRef,
			FailureReasonCode:      item.FailureReasonCode,
		})
	}
	return records
}
