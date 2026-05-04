package bridge

import (
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestReferenceFromTouchBrowserEvidenceMapsSourceRisk(t *testing.T) {
	cases := []contracts.SourceRisk{
		contracts.SourceRiskLow,
		contracts.SourceRiskMedium,
		contracts.SourceRiskHostile,
	}
	for _, sourceRisk := range cases {
		reference, err := ReferenceFromTouchBrowserEvidence(TouchBrowserEvidence{
			EvidenceRef: "tc://evidence/browser-1",
			Title:       "Browser evidence",
			SourceRisk:  sourceRisk,
		})
		if err != nil {
			t.Fatalf("map source risk %s: %v", sourceRisk, err)
		}
		if reference.Ref != "tc://evidence/browser-1" || reference.Type != ReferenceTypeEvidence || reference.SourceRisk != sourceRisk {
			t.Fatalf("unexpected evidence reference for source risk %s: %+v", sourceRisk, reference)
		}
	}
}

func TestReferenceFromTouchBrowserEvidenceRejectsInvalidInput(t *testing.T) {
	if _, err := ReferenceFromTouchBrowserEvidence(TouchBrowserEvidence{}); err == nil {
		t.Fatalf("expected missing evidence_ref to fail")
	}
	if _, err := ReferenceFromTouchBrowserEvidence(TouchBrowserEvidence{
		EvidenceRef: "tc://evidence/browser-1",
		SourceRisk:  contracts.SourceRisk("unknown"),
	}); err == nil {
		t.Fatalf("expected invalid source_risk to fail")
	}
}

func TestAttachTouchBrowserEvidenceAppendsWithoutMutatingOriginal(t *testing.T) {
	original := contracts.MessageIngressRequest{
		Payload: contracts.Payload{
			Summary: "handoff",
			Body:    "use browser evidence",
			References: []contracts.Reference{
				{Ref: "tc://artifact/art_1", Type: "artifact"},
			},
		},
	}
	updated, err := AttachTouchBrowserEvidence(original, TouchBrowserEvidence{
		EvidenceRef: "tc://evidence/browser-2",
		URL:         "https://example.test/evidence",
		SourceRisk:  contracts.SourceRiskMedium,
	})
	if err != nil {
		t.Fatalf("attach evidence: %v", err)
	}
	if len(original.Payload.References) != 1 {
		t.Fatalf("expected original request to keep one reference, got %+v", original.Payload.References)
	}
	if len(updated.Payload.References) != 2 {
		t.Fatalf("expected updated request to append evidence, got %+v", updated.Payload.References)
	}
	evidence := updated.Payload.References[1]
	if evidence.Ref != "tc://evidence/browser-2" || evidence.Type != ReferenceTypeEvidence || evidence.SourceRisk != contracts.SourceRiskMedium {
		t.Fatalf("unexpected attached evidence reference: %+v", evidence)
	}
}
