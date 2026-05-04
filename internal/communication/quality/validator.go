package quality

import (
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	RuleMissingRequiredField    = "missing_required_field"
	RuleMissingReadbackTarget   = "missing_readback_target"
	RuleMissingLineageReference = "missing_lineage_reference"

	DefaultPolicyRef     = "tc://quality-policy/default"
	DefaultPolicyVersion = "v0"
)

var defaultReadbackTargets = []string{"goal", "constraints", "next_action"}

type ValidationInput struct {
	DecisionRef string
	MessageRef  string
	Request     contracts.MessageIngressRequest
	CreatedAt   time.Time
	CreatedBy   string
}

func ValidateMessage(input ValidationInput) contracts.QualityDecision {
	policy := effectivePolicy(input.Request)
	decision := contracts.QualityDecision{
		QualityDecisionRef: input.DecisionRef,
		MessageRef:         input.MessageRef,
		PolicyRef:          policy.PolicyRef,
		PolicyVersion:      policy.PolicyVersion,
		Decision:           contracts.QualityDecisionPassed,
		FallbackAction:     policy.FallbackAction,
		CreatedBy:          input.CreatedBy,
	}
	if !input.CreatedAt.IsZero() {
		decision.CreatedAt = input.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	decision.Violations = append(decision.Violations, missingRequiredFieldViolations(input.Request, policy)...)
	if violation, ok := missingReadbackTargetViolation(policy); ok {
		decision.Violations = append(decision.Violations, violation)
	}
	if violation, ok := missingLineageReferenceViolation(input.Request); ok {
		decision.Violations = append(decision.Violations, violation)
	}
	decision.Decision = decisionStatus(policy, decision.Violations)
	return decision
}

func effectivePolicy(req contracts.MessageIngressRequest) contracts.PhraseologyPolicy {
	if req.PhraseologyPolicy != nil {
		policy := *req.PhraseologyPolicy
		if policy.PolicyRef == "" {
			policy.PolicyRef = DefaultPolicyRef
		}
		if policy.PolicyVersion == "" {
			policy.PolicyVersion = DefaultPolicyVersion
		}
		if policy.FallbackAction == "" {
			policy.FallbackAction = contracts.QualityFallbackWarn
		}
		if policy.Severity == "" {
			policy.Severity = contracts.QualitySeverityWarning
		}
		return policy
	}
	policy := contracts.PhraseologyPolicy{
		PolicyRef:      DefaultPolicyRef,
		PolicyVersion:  DefaultPolicyVersion,
		ScopeKind:      "global",
		FallbackAction: contracts.QualityFallbackWarn,
		Severity:       contracts.QualitySeverityWarning,
		AuditMode:      "append",
	}
	if req.ReadbackRequired {
		policy.Readback = contracts.PhraseologyReadbackPolicy{Required: true, Targets: append([]string(nil), defaultReadbackTargets...)}
	}
	return policy
}

func missingRequiredFieldViolations(req contracts.MessageIngressRequest, policy contracts.PhraseologyPolicy) []contracts.QualityViolation {
	violations := make([]contracts.QualityViolation, 0)
	for _, field := range policy.RequiredFields {
		field = strings.TrimSpace(field)
		if field == "" || fieldPresent(req, field) {
			continue
		}
		violations = append(violations, contracts.QualityViolation{
			Code:         RuleMissingRequiredField,
			Field:        field,
			Detail:       "required field is not present in the message envelope, payload, constraints, or references",
			Severity:     severity(policy),
			SuggestedFix: "add the required field or remove it from the PhraseologyPolicy required_fields list",
		})
	}
	return violations
}

func missingReadbackTargetViolation(policy contracts.PhraseologyPolicy) (contracts.QualityViolation, bool) {
	if !policy.Readback.Required {
		return contracts.QualityViolation{}, false
	}
	for _, target := range policy.Readback.Targets {
		if strings.TrimSpace(target) != "" {
			return contracts.QualityViolation{}, false
		}
	}
	return contracts.QualityViolation{
		Code:         RuleMissingReadbackTarget,
		Field:        "readback.targets",
		Detail:       "readback is required but no readback target field set was configured",
		Severity:     severity(policy),
		SuggestedFix: "set readback.targets or rely on the default readback policy",
	}, true
}

func missingLineageReferenceViolation(req contracts.MessageIngressRequest) (contracts.QualityViolation, bool) {
	if !mentionsLineageIntent(req.Payload.Body) {
		return contracts.QualityViolation{}, false
	}
	if hasLineageReference(req.Payload.References) {
		return contracts.QualityViolation{}, false
	}
	return contracts.QualityViolation{
		Code:         RuleMissingLineageReference,
		Field:        "payload.references",
		Detail:       "message appears to modify, replace, or derive from an artifact but has no artifact lineage reference",
		Severity:     contracts.QualitySeverityWarning,
		SuggestedFix: "add an artifact or artifact_version reference that captures parent_version, derived_from, or supersedes",
	}, true
}

func fieldPresent(req contracts.MessageIngressRequest, field string) bool {
	switch strings.TrimSpace(field) {
	case "message_ref":
		return req.MessageRef != ""
	case "sender_endpoint_ref":
		return req.SenderEndpointRef != ""
	case "target_capability":
		return req.TargetCapability != ""
	case "correlation_ref":
		return req.CorrelationRef != ""
	case "readback_required":
		return req.ReadbackRequired
	case "payload.summary":
		return strings.TrimSpace(req.Payload.Summary) != ""
	case "payload.body":
		return strings.TrimSpace(req.Payload.Body) != ""
	case "payload.references", "references":
		return len(req.Payload.References) > 0
	case "constraints":
		return len(req.Constraints) > 0
	default:
		return fieldInConstraintsOrReferences(req, field)
	}
}

func fieldInConstraintsOrReferences(req contracts.MessageIngressRequest, field string) bool {
	for _, constraint := range req.Constraints {
		if constraint.Code == field || constraint.SourceRef == field {
			return true
		}
	}
	for _, ref := range req.Payload.References {
		if ref.Ref == field || ref.Type == field {
			return true
		}
	}
	return false
}

func mentionsLineageIntent(body string) bool {
	body = strings.ToLower(body)
	patterns := []string{
		"수정",
		"대체",
		"파생",
		"기반",
		"replace",
		"based on",
		"derive",
		"derived from",
		"supersede",
		"modify artifact",
	}
	for _, pattern := range patterns {
		if strings.Contains(body, pattern) {
			return true
		}
	}
	return false
}

func hasLineageReference(references []contracts.Reference) bool {
	for _, ref := range references {
		refType := strings.ToLower(ref.Type)
		refValue := strings.ToLower(ref.Ref)
		if refType == "artifact" || refType == "artifact_version" || strings.Contains(refValue, "artifact-version") || strings.Contains(refValue, "artifact/") {
			return true
		}
	}
	return false
}

func severity(policy contracts.PhraseologyPolicy) string {
	if policy.Severity != "" {
		return policy.Severity
	}
	return contracts.QualitySeverityWarning
}

func decisionStatus(policy contracts.PhraseologyPolicy, violations []contracts.QualityViolation) string {
	if len(violations) == 0 {
		return contracts.QualityDecisionPassed
	}
	switch policy.FallbackAction {
	case contracts.QualityFallbackReject:
		return contracts.QualityDecisionRejected
	case contracts.QualityFallbackRequestClarification:
		return contracts.QualityDecisionClarificationRequired
	case contracts.QualityFallbackRouteToReview:
		return contracts.QualityDecisionReviewRequired
	default:
		return contracts.QualityDecisionWarned
	}
}
