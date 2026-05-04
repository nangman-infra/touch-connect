package cli

import (
	"encoding/json"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func parseConstraint(value string) contracts.Constraint {
	var raw struct {
		Code    string `json:"code"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(value), &raw); err == nil && raw.Code != "" {
		return contracts.Constraint{Code: raw.Code, Summary: raw.Summary}
	}
	for index, char := range value {
		if char == ':' {
			return contracts.Constraint{Code: value[:index], Summary: value[index+1:]}
		}
	}
	return contracts.Constraint{Code: value, Summary: value}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
