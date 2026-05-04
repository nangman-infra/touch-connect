package application

import "github.com/nangman-infra/touch-connect/tc-server/internal/domain"

func attemptClosed(state string) bool {
	switch state {
	case domain.AttemptStateCompleted, domain.AttemptStateFailed, domain.AttemptStateOrphaned, domain.AttemptStateCanceled:
		return true
	default:
		return false
	}
}
