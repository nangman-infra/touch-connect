package contracts

const (
	QualityDecisionPassed                = "passed"
	QualityDecisionWarned                = "warned"
	QualityDecisionRejected              = "rejected"
	QualityDecisionClarificationRequired = "clarification_required"
	QualityDecisionReviewRequired        = "review_required"
	QualityDecisionSkipped               = "skipped"

	QualityFallbackReject               = "reject"
	QualityFallbackWarn                 = "warn"
	QualityFallbackRequestClarification = "request_clarification"
	QualityFallbackRouteToReview        = "route_to_review"

	QualitySeverityInfo     = "info"
	QualitySeverityWarning  = "warning"
	QualitySeverityBlocking = "blocking"
)

type PhraseologyPolicy struct {
	PolicyRef             string                    `json:"policy_ref"`
	PolicyVersion         string                    `json:"policy_version"`
	ScopeKind             string                    `json:"scope_kind"`
	ScopeRef              string                    `json:"scope_ref,omitempty"`
	AppliesToCapabilities []CapabilityClaim         `json:"applies_to_capabilities,omitempty"`
	RequiredFields        []string                  `json:"required_fields,omitempty"`
	Readback              PhraseologyReadbackPolicy `json:"readback,omitempty"`
	ConstraintRules       []PhraseologyRule         `json:"constraint_rules,omitempty"`
	AmbiguityRules        []PhraseologyRule         `json:"ambiguity_rules,omitempty"`
	FallbackAction        string                    `json:"fallback_action,omitempty"`
	Severity              string                    `json:"severity,omitempty"`
	AuditMode             string                    `json:"audit_mode,omitempty"`
}

type PhraseologyReadbackPolicy struct {
	Required bool     `json:"required,omitempty"`
	Targets  []string `json:"targets,omitempty"`
}

type PhraseologyRule struct {
	Code         string `json:"code"`
	Field        string `json:"field,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	Severity     string `json:"severity,omitempty"`
	Fallback     string `json:"fallback,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

type CapabilityClaim struct {
	ClaimRef           string                 `json:"claim_ref,omitempty"`
	Capabilities       []CapabilityClaimItem  `json:"capabilities,omitempty"`
	VersionConstraints []string               `json:"version_constraints,omitempty"`
	Scope              string                 `json:"scope,omitempty"`
	FallbackChain      []CapabilityClaimItem  `json:"fallback_chain,omitempty"`
	RequiredEvidence   []RequiredEvidenceItem `json:"required_evidence,omitempty"`
}

type CapabilityClaimItem struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type RequiredEvidenceItem struct {
	Ref   string `json:"ref,omitempty"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

type QualityDecision struct {
	QualityDecisionRef string             `json:"quality_decision_ref"`
	MessageRef         string             `json:"message_ref"`
	PolicyRef          string             `json:"policy_ref,omitempty"`
	PolicyVersion      string             `json:"policy_version,omitempty"`
	Decision           string             `json:"decision"`
	Violations         []QualityViolation `json:"violations,omitempty"`
	FallbackAction     string             `json:"fallback_action,omitempty"`
	CreatedAt          string             `json:"created_at,omitempty"`
	CreatedBy          string             `json:"created_by,omitempty"`
}

type QualityViolation struct {
	Code         string `json:"code"`
	Field        string `json:"field,omitempty"`
	Detail       string `json:"detail"`
	Severity     string `json:"severity,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}
