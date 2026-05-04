package contracts

import "fmt"

const ErrorCodeQualityRejected = "quality_rejected"

type APIError struct {
	StatusCode int
	Response   ErrorResponse
}

func (e APIError) Error() string {
	if e.Response.Code != "" {
		return fmt.Sprintf("%s: %s", e.Response.Code, e.Response.Message)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("api_status_%d", e.StatusCode)
	}
	return "api_error"
}
