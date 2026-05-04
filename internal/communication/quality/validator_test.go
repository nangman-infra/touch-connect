package quality

import (
	"testing"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestValidateMessageUsesDefaultReadbackPolicyForBooleanProjection(t *testing.T) {
	decision := ValidateMessage(ValidationInput{
		DecisionRef: "tc://quality-decision/qdc_000001",
		MessageRef:  "tc://message/msg_quality_default",
		Request: contracts.MessageIngressRequest{
			SenderEndpointRef: "tc://endpoint/tcctl",
			TargetCapability:  "code.change",
			ReadbackRequired:  true,
			Payload: contracts.Payload{
				Summary:    "change code",
				Body:       "make a small change",
				References: []contracts.Reference{},
			},
			Constraints: []contracts.Constraint{},
		},
		CreatedAt: time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
		CreatedBy: "tc://endpoint/tcctl",
	})
	if decision.Decision != contracts.QualityDecisionPassed {
		t.Fatalf("expected passed default decision, got %+v", decision)
	}
	if decision.PolicyRef != DefaultPolicyRef || decision.PolicyVersion != DefaultPolicyVersion {
		t.Fatalf("expected default policy identity, got %+v", decision)
	}
}

func TestValidateMessageHonorsExplicitPolicyOverBooleanProjection(t *testing.T) {
	policy := &contracts.PhraseologyPolicy{
		PolicyRef:      "tc://quality-policy/explicit",
		PolicyVersion:  "1",
		ScopeKind:      "task",
		FallbackAction: contracts.QualityFallbackReject,
		Severity:       contracts.QualitySeverityBlocking,
		Readback:       contracts.PhraseologyReadbackPolicy{Required: true},
	}
	decision := ValidateMessage(ValidationInput{
		DecisionRef: "tc://quality-decision/qdc_000002",
		MessageRef:  "tc://message/msg_quality_explicit",
		Request: contracts.MessageIngressRequest{
			SenderEndpointRef: "tc://endpoint/tcctl",
			TargetCapability:  "code.change",
			ReadbackRequired:  true,
			PhraseologyPolicy: policy,
			Payload: contracts.Payload{
				Summary:    "change code",
				Body:       "make a small change",
				References: []contracts.Reference{},
			},
			Constraints: []contracts.Constraint{},
		},
	})
	if decision.Decision != contracts.QualityDecisionRejected {
		t.Fatalf("expected explicit policy to drive rejected decision, got %+v", decision)
	}
	if len(decision.Violations) != 1 || decision.Violations[0].Code != RuleMissingReadbackTarget {
		t.Fatalf("expected missing readback target violation, got %+v", decision.Violations)
	}
}

func TestValidateMessageCatchesMissingRequiredField(t *testing.T) {
	policy := &contracts.PhraseologyPolicy{
		PolicyRef:      "tc://quality-policy/required",
		PolicyVersion:  "1",
		ScopeKind:      "capability",
		RequiredFields: []string{"constraints"},
		FallbackAction: contracts.QualityFallbackWarn,
	}
	decision := ValidateMessage(ValidationInput{
		DecisionRef: "tc://quality-decision/qdc_000003",
		MessageRef:  "tc://message/msg_quality_required",
		Request: contracts.MessageIngressRequest{
			SenderEndpointRef: "tc://endpoint/tcctl",
			TargetCapability:  "code.change",
			PhraseologyPolicy: policy,
			Payload: contracts.Payload{
				Summary:    "change code",
				Body:       "make a small change",
				References: []contracts.Reference{},
			},
			Constraints: []contracts.Constraint{},
		},
	})
	if decision.Decision != contracts.QualityDecisionWarned {
		t.Fatalf("expected warned decision, got %+v", decision)
	}
	if len(decision.Violations) != 1 || decision.Violations[0].Code != RuleMissingRequiredField {
		t.Fatalf("expected missing required field violation, got %+v", decision.Violations)
	}
}

func TestValidateMessageCatchesMissingLineageReference(t *testing.T) {
	decision := ValidateMessage(ValidationInput{
		DecisionRef: "tc://quality-decision/qdc_000004",
		MessageRef:  "tc://message/msg_quality_lineage",
		Request: contracts.MessageIngressRequest{
			SenderEndpointRef: "tc://endpoint/tcctl",
			TargetCapability:  "code.change",
			Payload: contracts.Payload{
				Summary:    "replace artifact",
				Body:       "replace the prior artifact with this new version",
				References: []contracts.Reference{},
			},
			Constraints: []contracts.Constraint{},
		},
	})
	if decision.Decision != contracts.QualityDecisionWarned {
		t.Fatalf("expected warned decision, got %+v", decision)
	}
	if len(decision.Violations) != 1 || decision.Violations[0].Code != RuleMissingLineageReference {
		t.Fatalf("expected missing lineage reference violation, got %+v", decision.Violations)
	}
}
