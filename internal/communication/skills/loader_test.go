package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestLoadFileParsesGuidanceSkillMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frontend-design", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	body := `---
skill_ref: tc://skill/frontend-design-review
name: Frontend Design Review
kind: guidance
description: Apply product UI judgment before implementation.
capabilities:
  - frontend.design-review
  - ui.consistency-check
applies_to: [React, Next.js]
approval_required: true
executor_hint: llm
---
# Frontend Design Review

Check layout geometry before copy. Verify mobile and desktop.
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	skill, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load skill: %v", err)
	}
	if skill.SkillRef != "tc://skill/frontend-design-review" || skill.Kind != contracts.SkillKindGuidance {
		t.Fatalf("unexpected skill identity: %+v", skill)
	}
	if len(skill.Capabilities) != 2 || skill.Capabilities[0] != "frontend.design-review" {
		t.Fatalf("expected capabilities from frontmatter, got %+v", skill.Capabilities)
	}
	if len(skill.AppliesTo) != 2 || !skill.ApprovalRequired || skill.ExecutorHint != "llm" {
		t.Fatalf("expected guidance metadata, got %+v", skill)
	}
}

func TestLoadFileInfersDesignLikeMarkdownDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "design-review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# Design Review\n\nUse this guidance before changing UI.\n"), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	skill, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load inferred skill: %v", err)
	}
	if skill.SkillRef != "tc://skill/design-review" || skill.Kind != contracts.SkillKindGuidance || skill.Name != "design review" {
		t.Fatalf("expected inferred guidance skill, got %+v", skill)
	}
}
