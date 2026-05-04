package contracts

import "fmt"

type PolicyDecision string

const (
	PolicyDecisionAllow  PolicyDecision = "allow"
	PolicyDecisionReview PolicyDecision = "review"
	PolicyDecisionBlock  PolicyDecision = "block"
)

func (d PolicyDecision) String() string {
	return string(d)
}

func (d PolicyDecision) Valid() bool {
	switch d {
	case PolicyDecisionAllow, PolicyDecisionReview, PolicyDecisionBlock:
		return true
	default:
		return false
	}
}

func ParsePolicyDecision(value string) (PolicyDecision, error) {
	decision := PolicyDecision(value)
	if !decision.Valid() {
		return "", fmt.Errorf("invalid policy decision %q", value)
	}
	return decision, nil
}

type RiskClass string

const (
	RiskClassLow     RiskClass = "low"
	RiskClassHigh    RiskClass = "high"
	RiskClassBlocked RiskClass = "blocked"
)

func (c RiskClass) String() string {
	return string(c)
}

func (c RiskClass) Valid() bool {
	switch c {
	case RiskClassLow, RiskClassHigh, RiskClassBlocked:
		return true
	default:
		return false
	}
}

func ParseRiskClass(value string) (RiskClass, error) {
	class := RiskClass(value)
	if !class.Valid() {
		return "", fmt.Errorf("invalid risk class %q", value)
	}
	return class, nil
}

type SourceRisk string

const (
	SourceRiskLow     SourceRisk = "low"
	SourceRiskMedium  SourceRisk = "medium"
	SourceRiskHostile SourceRisk = "hostile"
)

func (r SourceRisk) String() string {
	return string(r)
}

func (r SourceRisk) Valid() bool {
	switch r {
	case SourceRiskLow, SourceRiskMedium, SourceRiskHostile:
		return true
	default:
		return false
	}
}

func ParseSourceRisk(value string) (SourceRisk, error) {
	risk := SourceRisk(value)
	if !risk.Valid() {
		return "", fmt.Errorf("invalid source risk %q", value)
	}
	return risk, nil
}

// ConfidenceBand on readback/checkpoint/message quality reflects PhraseologyPolicy
// compliance, not evidence-supported certainty. When evidence flows through a
// future touch-browser bridge, browser confidenceBand maps to reference-level
// evidence metadata, not directly into this message-level band.
type ConfidenceBand string

const (
	ConfidenceBandHigh   ConfidenceBand = "high"
	ConfidenceBandMedium ConfidenceBand = "medium"
	ConfidenceBandReview ConfidenceBand = "review"
)

func (b ConfidenceBand) String() string {
	return string(b)
}

func (b ConfidenceBand) Valid() bool {
	switch b {
	case ConfidenceBandHigh, ConfidenceBandMedium, ConfidenceBandReview:
		return true
	default:
		return false
	}
}

func ParseConfidenceBand(value string) (ConfidenceBand, error) {
	band := ConfidenceBand(value)
	if !band.Valid() {
		return "", fmt.Errorf("invalid confidence band %q", value)
	}
	return band, nil
}

type QualityGateMode string

const (
	QualityGateEnforce QualityGateMode = "enforce"
	QualityGateWarn    QualityGateMode = "warn"
	QualityGateSkip    QualityGateMode = "skip"
)

func (m QualityGateMode) String() string {
	return string(m)
}

func (m QualityGateMode) Valid() bool {
	switch m {
	case QualityGateEnforce, QualityGateWarn, QualityGateSkip:
		return true
	default:
		return false
	}
}

func ParseQualityGateMode(value string) (QualityGateMode, error) {
	mode := QualityGateMode(value)
	if !mode.Valid() {
		return "", fmt.Errorf("invalid quality gate mode %q", value)
	}
	return mode, nil
}
