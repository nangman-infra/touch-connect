package bridge

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const ReferenceTypeEvidence = "evidence"

type TouchBrowserEvidence struct {
	EvidenceRef string               `json:"evidence_ref"`
	Type        string               `json:"type,omitempty"`
	Title       string               `json:"title,omitempty"`
	URL         string               `json:"url,omitempty"`
	Version     int                  `json:"version,omitempty"`
	SourceRisk  contracts.SourceRisk `json:"source_risk,omitempty"`
}

func ReferenceFromTouchBrowserEvidence(evidence TouchBrowserEvidence) (contracts.Reference, error) {
	if strings.TrimSpace(evidence.EvidenceRef) == "" {
		return contracts.Reference{}, errors.New("touchbrowser evidence_ref is required")
	}
	if evidence.SourceRisk != "" && !evidence.SourceRisk.Valid() {
		return contracts.Reference{}, fmt.Errorf("invalid touchbrowser source_risk %q", evidence.SourceRisk)
	}
	refType := strings.TrimSpace(evidence.Type)
	if refType == "" {
		refType = ReferenceTypeEvidence
	}
	title := strings.TrimSpace(evidence.Title)
	if title == "" {
		title = strings.TrimSpace(evidence.URL)
	}
	return contracts.Reference{
		Ref:        evidence.EvidenceRef,
		Type:       refType,
		Title:      title,
		Version:    evidence.Version,
		SourceRisk: evidence.SourceRisk,
	}, nil
}

func AttachTouchBrowserEvidence(req contracts.MessageIngressRequest, evidence ...TouchBrowserEvidence) (contracts.MessageIngressRequest, error) {
	if len(evidence) == 0 {
		return req, nil
	}
	references := append([]contracts.Reference(nil), req.Payload.References...)
	for _, item := range evidence {
		reference, err := ReferenceFromTouchBrowserEvidence(item)
		if err != nil {
			return contracts.MessageIngressRequest{}, err
		}
		references = append(references, reference)
	}
	req.Payload.References = references
	return req, nil
}
