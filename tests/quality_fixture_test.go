package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	"github.com/nangman-infra/touch-connect/internal/communication/quality"
)

type qualityFixture struct {
	Name          string                          `json:"name"`
	Rule          string                          `json:"rule"`
	Request       contracts.MessageIngressRequest `json:"request"`
	Policy        *contracts.PhraseologyPolicy    `json:"policy,omitempty"`
	WantViolation bool                            `json:"want_violation"`
	WantDecision  string                          `json:"want_decision"`
}

func TestQualityValidatorFixtures(t *testing.T) {
	fixtures := []string{
		"missing_required_field/passes.json",
		"missing_required_field/violates.json",
		"missing_required_field/edge.json",
		"missing_readback_target/passes.json",
		"missing_readback_target/violates.json",
		"missing_readback_target/edge.json",
		"missing_lineage_reference/passes.json",
		"missing_lineage_reference/violates.json",
		"missing_lineage_reference/edge.json",
		"missing_lineage_reference/false_positive_korean.json",
		"missing_lineage_reference/false_positive_basis.json",
		"missing_lineage_reference/false_positive_derive.json",
	}
	for _, fixturePath := range fixtures {
		t.Run(fixturePath, func(t *testing.T) {
			fixture := loadQualityFixture(t, fixturePath)
			fixture.Request.PhraseologyPolicy = fixture.Policy
			decision := quality.ValidateMessage(quality.ValidationInput{
				DecisionRef: "tc://quality-decision/qdc_fixture",
				MessageRef:  "tc://message/msg_fixture",
				Request:     fixture.Request,
				CreatedBy:   fixture.Request.SenderEndpointRef,
			})
			if decision.Decision != fixture.WantDecision {
				t.Fatalf("expected decision %q, got %+v", fixture.WantDecision, decision)
			}
			found := false
			for _, violation := range decision.Violations {
				if violation.Code == fixture.Rule {
					found = true
				}
			}
			if found != fixture.WantViolation {
				t.Fatalf("expected violation=%v for %s, got decision=%+v", fixture.WantViolation, fixture.Rule, decision)
			}
		})
	}
}

func loadQualityFixture(t *testing.T, name string) qualityFixture {
	t.Helper()
	path := filepath.Join("fixtures", "quality", name)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var fixture qualityFixture
	if err := json.Unmarshal(body, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	return fixture
}
