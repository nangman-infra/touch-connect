package contracts

import "testing"

func TestPolicyEnumsParseKnownValues(t *testing.T) {
	if value, err := ParsePolicyDecision("allow"); err != nil || value != PolicyDecisionAllow {
		t.Fatalf("parse policy decision: value=%q err=%v", value, err)
	}
	if value, err := ParseRiskClass("blocked"); err != nil || value != RiskClassBlocked {
		t.Fatalf("parse risk class: value=%q err=%v", value, err)
	}
	if value, err := ParseSourceRisk("hostile"); err != nil || value != SourceRiskHostile {
		t.Fatalf("parse source risk: value=%q err=%v", value, err)
	}
	if value, err := ParseConfidenceBand("review"); err != nil || value != ConfidenceBandReview {
		t.Fatalf("parse confidence band: value=%q err=%v", value, err)
	}
}

func TestPolicyEnumsRejectUnknownValues(t *testing.T) {
	if _, err := ParsePolicyDecision("maybe"); err == nil {
		t.Fatal("expected invalid policy decision error")
	}
	if _, err := ParseRiskClass("medium"); err == nil {
		t.Fatal("expected invalid risk class error")
	}
	if _, err := ParseSourceRisk("blocked"); err == nil {
		t.Fatal("expected invalid source risk error")
	}
	if _, err := ParseConfidenceBand("low"); err == nil {
		t.Fatal("expected invalid confidence band error")
	}
}
