package application

import (
	"context"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func (s *Service) Capabilities(ctx context.Context) (map[string][]string, error) {
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	index := map[string][]string{}
	for _, endpoint := range endpoints {
		for name := range endpoint.Capabilities {
			index[name] = append(index[name], endpoint.EndpointRef)
		}
	}
	return index, nil
}

func (s *Service) Attempts(ctx context.Context, taskRef string) ([]contracts.AttemptRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if taskRef == "" {
		return snapshot.Attempts, nil
	}
	messages := map[string]bool{}
	for _, item := range snapshot.Messages {
		if item.CorrelationRef == taskRef {
			messages[item.MessageRef] = true
		}
	}
	items := make([]contracts.AttemptRecord, 0)
	for _, item := range snapshot.Attempts {
		if messages[item.MessageRef] {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *Service) TaskStatus(ctx context.Context, taskRef string) (map[string]any, bool, error) {
	messages, err := s.Messages(ctx, taskRef)
	if err != nil {
		return nil, false, err
	}
	if len(messages) == 0 {
		return nil, false, nil
	}
	counts := map[string]int{}
	for _, message := range messages {
		counts[message.State]++
	}
	return map[string]any{
		"task_ref":       taskRef,
		"message_count":  len(messages),
		"state_counts":   counts,
		"latest_message": messages[len(messages)-1],
	}, true, nil
}

func (s *Service) TaskHistory(ctx context.Context, taskRef string) (map[string]any, bool, error) {
	messages, err := s.Messages(ctx, taskRef)
	if err != nil {
		return nil, false, err
	}
	if len(messages) == 0 {
		return nil, false, nil
	}
	attempts, err := s.Attempts(ctx, taskRef)
	if err != nil {
		return nil, false, err
	}
	artifacts, err := s.Artifacts(ctx, taskRef)
	if err != nil {
		return nil, false, err
	}
	return map[string]any{
		"task_ref":  taskRef,
		"messages":  messages,
		"attempts":  attempts,
		"artifacts": artifacts,
	}, true, nil
}

func (s *Service) Approvals(ctx context.Context) ([]contracts.ApprovalRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	return snapshot.Approvals, err
}

func (s *Service) Approval(ctx context.Context, ref string) (contracts.ApprovalRecord, bool, error) {
	items, err := s.Approvals(ctx)
	if err != nil {
		return contracts.ApprovalRecord{}, false, err
	}
	for _, item := range items {
		if item.ApprovalRef == ref {
			return item, true, nil
		}
	}
	return contracts.ApprovalRecord{}, false, nil
}

func (s *Service) DeadLetters(ctx context.Context) ([]contracts.DeadLetterRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	return snapshot.DeadLetters, err
}

func (s *Service) DeadLetter(ctx context.Context, ref string) (contracts.DeadLetterRecord, bool, error) {
	items, err := s.DeadLetters(ctx)
	if err != nil {
		return contracts.DeadLetterRecord{}, false, err
	}
	for _, item := range items {
		if item.DeadLetterRef == ref {
			return item, true, nil
		}
	}
	return contracts.DeadLetterRecord{}, false, nil
}

func (s *Service) SideEffects(ctx context.Context, taskRef string) ([]contracts.SideEffectRecord, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	if taskRef == "" {
		return snapshot.SideEffects, nil
	}
	items := make([]contracts.SideEffectRecord, 0)
	for _, item := range snapshot.SideEffects {
		if item.TaskRef == taskRef {
			items = append(items, item)
		}
	}
	return items, nil
}
