package application

import (
	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/tc-server/internal/domain"
)

func (s *Service) CancelTask(req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	if req.TaskRef == "" {
		return contracts.TaskCommandResponse{}, domain.ErrInvalidInput
	}
	messages := s.messagesForTask(req.TaskRef)
	if len(messages) == 0 {
		return contracts.TaskCommandResponse{}, domain.ErrMessageNotFound
	}
	response := contracts.TaskCommandResponse{TaskRef: req.TaskRef, State: domain.MessageStateCanceled}
	for _, message := range messages {
		if message.State == domain.MessageStateCompleted || message.State == domain.MessageStateCanceled {
			continue
		}
		message.State = domain.MessageStateCanceled
		if err := s.messages.UpdateMessage(message); err != nil {
			return contracts.TaskCommandResponse{}, err
		}
		response.MessageRefs = append(response.MessageRefs, message.MessageRef)
		response.AffectedMessages++
		if message.AttemptRef != "" {
			if attempt, ok := s.store.GetAttempt(message.AttemptRef); ok && !attemptClosed(attempt.State) {
				attempt.State = domain.AttemptStateCanceled
				if err := s.store.UpdateAttempt(attempt); err != nil {
					return contracts.TaskCommandResponse{}, err
				}
				response.AttemptRefs = append(response.AttemptRefs, attempt.AttemptRef)
				response.AffectedAttempts++
			}
		}
	}
	return response, nil
}

func (s *Service) RetryTask(req contracts.TaskCommandRequest) (contracts.TaskCommandResponse, error) {
	if req.TaskRef == "" {
		return contracts.TaskCommandResponse{}, domain.ErrInvalidInput
	}
	messages := s.messagesForTask(req.TaskRef)
	if len(messages) == 0 {
		return contracts.TaskCommandResponse{}, domain.ErrMessageNotFound
	}
	response := contracts.TaskCommandResponse{TaskRef: req.TaskRef, State: domain.MessageStateAvailable}
	for _, message := range messages {
		if message.State != domain.MessageStateFailed &&
			message.State != domain.MessageStateDeadLettered &&
			message.State != domain.MessageStateInputRequired &&
			message.State != domain.MessageStateCanceled {
			continue
		}
		message.State = domain.MessageStateAvailable
		message.AttemptRef = ""
		if err := s.messages.UpdateMessage(message); err != nil {
			return contracts.TaskCommandResponse{}, err
		}
		response.MessageRefs = append(response.MessageRefs, message.MessageRef)
		response.AffectedMessages++
	}
	return response, nil
}

func (s *Service) ReplayDeadLetter(req contracts.DLQReplayRequest) (contracts.DLQReplayResponse, error) {
	if req.DeadLetterRef == "" {
		return contracts.DLQReplayResponse{}, domain.ErrInvalidInput
	}
	deadLetter, ok := s.deadLetter(req.DeadLetterRef)
	if !ok {
		return contracts.DLQReplayResponse{}, domain.ErrMessageNotFound
	}
	original, ok := s.messages.GetMessage(deadLetter.MessageRef)
	if !ok {
		return contracts.DLQReplayResponse{}, domain.ErrMessageNotFound
	}
	replayed, err := s.IngressMessage(contracts.MessageIngressRequest{
		SenderEndpointRef: original.SenderEndpointRef,
		TargetCapability:  original.TargetCapability,
		Payload:           original.Payload,
		Constraints:       original.Constraints,
		CorrelationRef:    original.CorrelationRef,
		ReadbackRequired:  original.ReadbackRequired,
	})
	if err != nil {
		return contracts.DLQReplayResponse{}, err
	}
	return contracts.DLQReplayResponse{
		DeadLetterRef: req.DeadLetterRef,
		OriginalRef:   original.MessageRef,
		MessageRef:    replayed.MessageRef,
		DeliveryRef:   replayed.DeliveryRef,
		State:         replayed.State,
	}, nil
}

func (s *Service) messagesForTask(taskRef string) []domain.Message {
	snapshot := s.Snapshot()
	messages := make([]domain.Message, 0)
	for _, message := range snapshot.Messages {
		if message.CorrelationRef == taskRef {
			messages = append(messages, message)
		}
	}
	return messages
}

func (s *Service) deadLetter(deadLetterRef string) (domain.DeadLetter, bool) {
	snapshot := s.Snapshot()
	for _, item := range snapshot.DeadLetters {
		if item.DeadLetterRef == deadLetterRef {
			return item, true
		}
	}
	return domain.DeadLetter{}, false
}
