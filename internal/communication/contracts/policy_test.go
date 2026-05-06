package contracts

import "testing"

func TestPolicyEnumParsingAndString(t *testing.T) {
	enumCases := []struct {
		name  string
		value string
		parse func(string) (string, error)
	}{
		{"policy decision", "allow", func(v string) (string, error) {
			parsed, err := ParsePolicyDecision(v)
			return parsed.String(), err
		}},
		{"risk class", "high", func(v string) (string, error) {
			parsed, err := ParseRiskClass(v)
			return parsed.String(), err
		}},
		{"source risk", "medium", func(v string) (string, error) {
			parsed, err := ParseSourceRisk(v)
			return parsed.String(), err
		}},
		{"confidence band", "review", func(v string) (string, error) {
			parsed, err := ParseConfidenceBand(v)
			return parsed.String(), err
		}},
		{"quality gate", "warn", func(v string) (string, error) {
			parsed, err := ParseQualityGateMode(v)
			return parsed.String(), err
		}},
	}
	for _, tc := range enumCases {
		got, err := tc.parse(tc.value)
		if err != nil {
			t.Fatalf("%s parse returned error: %v", tc.name, err)
		}
		if got != tc.value {
			t.Fatalf("%s String() = %q, want %q", tc.name, got, tc.value)
		}
		if _, err := tc.parse("bad"); err == nil {
			t.Fatalf("%s should reject invalid value", tc.name)
		}
	}
}

func TestAPIError(t *testing.T) {
	cases := []struct {
		err  APIError
		want string
	}{
		{APIError{Response: ErrorResponse{Code: "quality_rejected", Message: "blocked"}}, "quality_rejected: blocked"},
		{APIError{StatusCode: 502}, "api_status_502"},
		{APIError{}, "api_error"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Fatalf("Error() = %q, want %q", got, tc.want)
		}
	}
}
