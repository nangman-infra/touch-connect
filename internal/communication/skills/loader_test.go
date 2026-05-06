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

func TestLoadDirAndRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, filepath.Join(dir, "alpha", "SKILL.md"), `---
skill_ref: tc://skill/alpha
name: Alpha
kind: executable
capabilities: [code.change]
---
# Alpha

Run code changes.
`)
	writeSkill(t, filepath.Join(dir, "beta", "SKILL.md"), `# Beta

Review the result.
`)

	loaded, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir returned error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected two skills, got %+v", loaded)
	}
	if matched, ok := MatchCapability(loaded, "code.change"); !ok || matched.SkillRef != "tc://skill/alpha" {
		t.Fatalf("expected capability match, got %+v ok=%v", matched, ok)
	}

	registry := contracts.SkillRegistry{}
	for _, skill := range loaded {
		var err error
		registry, err = Upsert(registry, skill)
		if err != nil {
			t.Fatalf("Upsert returned error: %v", err)
		}
	}
	path := filepath.Join(dir, "registry", "skills.json")
	if err := SaveRegistry(path, registry); err != nil {
		t.Fatalf("SaveRegistry returned error: %v", err)
	}
	readBack, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}
	if found, ok := Find(readBack, "Alpha"); !ok || found.SkillRef != "tc://skill/alpha" {
		t.Fatalf("expected to find Alpha, got %+v ok=%v", found, ok)
	}
}

func TestLoaderAndRegistryRejectRelativePaths(t *testing.T) {
	if _, err := LoadFile("relative/SKILL.md"); err == nil {
		t.Fatal("LoadFile should reject relative paths")
	}
	if _, err := LoadDir("relative"); err == nil {
		t.Fatal("LoadDir should reject relative paths")
	}
	if _, err := LoadRegistry("relative.json"); err == nil {
		t.Fatal("LoadRegistry should reject relative paths")
	}
	if err := SaveRegistry("relative.json", contracts.SkillRegistry{}); err == nil {
		t.Fatal("SaveRegistry should reject relative paths")
	}
}

func TestValidateRejectsInvalidSkillDefinitions(t *testing.T) {
	valid := contracts.SkillDefinition{
		SkillRef: "tc://skill/review",
		Name:     "Review",
		Kind:     contracts.SkillKindGuidance,
		Body:     "Review the work.",
	}
	if _, err := Validate(valid); err != nil {
		t.Fatalf("valid skill rejected: %v", err)
	}

	cases := map[string]func(*contracts.SkillDefinition){
		"missing_ref": func(skill *contracts.SkillDefinition) { skill.SkillRef = "" },
		"bad_ref":     func(skill *contracts.SkillDefinition) { skill.SkillRef = "skill/review" },
		"missing_name": func(skill *contracts.SkillDefinition) {
			skill.Name = ""
		},
		"bad_kind":     func(skill *contracts.SkillDefinition) { skill.Kind = "unknown" },
		"missing_body": func(skill *contracts.SkillDefinition) { skill.Body = "  " },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			skill := valid
			mutate(&skill)
			if _, err := Validate(skill); err == nil {
				t.Fatalf("%s should be rejected", name)
			}
		})
	}
}

func TestFrontMatterHelpersCoverListsBooleansAndSafeDefaults(t *testing.T) {
	meta := parseFrontMatter(`
# comment
name: "Quoted Name"
capabilities: [code.change, code.change, ai.review]
approval_required: yes
ignored line
applies_to:
  - 'Go'
  - "Terminal UI"
`)
	if meta.scalar("name") != "Quoted Name" {
		t.Fatalf("unexpected scalar: %q", meta.scalar("name"))
	}
	if got := meta.list("capabilities"); len(got) != 2 || got[0] != "code.change" || got[1] != "ai.review" {
		t.Fatalf("unexpected deduped capabilities: %+v", got)
	}
	if got := meta.list("applies_to"); len(got) != 2 || got[0] != "Go" || got[1] != "Terminal UI" {
		t.Fatalf("unexpected applies_to: %+v", got)
	}
	if !meta.bool("approval_required") {
		t.Fatal("approval_required yes should parse as true")
	}
	if meta.bool("missing") {
		t.Fatal("missing boolean should be false")
	}
	if safeRefPart("  !!!  ") != "local" {
		t.Fatal("empty safe ref should fall back to local")
	}
	if titleFromSlug("code_review-skill") != "code review skill" {
		t.Fatal("titleFromSlug should normalize separators")
	}
}

func TestRegistryUpsertReplacesAndSorts(t *testing.T) {
	registry := contracts.SkillRegistry{}
	var err error
	registry, err = Upsert(registry, contracts.SkillDefinition{
		SkillRef: "tc://skill/zeta",
		Name:     "Zeta",
		Kind:     contracts.SkillKindGuidance,
		Body:     "Zeta body",
	})
	if err != nil {
		t.Fatalf("upsert zeta: %v", err)
	}
	registry, err = Upsert(registry, contracts.SkillDefinition{
		SkillRef: "tc://skill/alpha",
		Name:     "Alpha",
		Kind:     contracts.SkillKindGuidance,
		Body:     "Alpha body",
	})
	if err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}
	registry, err = Upsert(registry, contracts.SkillDefinition{
		SkillRef: "tc://skill/zeta",
		Name:     "Zeta Updated",
		Kind:     contracts.SkillKindExecutable,
		Body:     "Updated body",
	})
	if err != nil {
		t.Fatalf("replace zeta: %v", err)
	}
	if len(registry.Skills) != 2 || registry.Skills[0].SkillRef != "tc://skill/alpha" || registry.Skills[1].Name != "Zeta Updated" {
		t.Fatalf("unexpected registry ordering/replacement: %+v", registry.Skills)
	}
	if _, err := Upsert(registry, contracts.SkillDefinition{}); err == nil {
		t.Fatal("invalid skill upsert should fail")
	}
}

func writeSkill(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
