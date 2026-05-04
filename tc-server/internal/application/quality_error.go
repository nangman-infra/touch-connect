package application

import "github.com/nangman-infra/touch-connect/internal/communication/contracts"

type QualityRejectedError struct {
	Decision contracts.QualityDecision
}

func (e QualityRejectedError) Error() string {
	return contracts.ErrorCodeQualityRejected
}
