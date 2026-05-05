package contracts

const (
	SkillKindGuidance   = "guidance"
	SkillKindExecutable = "executable"
)

type SkillDefinition struct {
	SkillRef         string   `json:"skill_ref"`
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	Description      string   `json:"description,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	AppliesTo        []string `json:"applies_to,omitempty"`
	ApprovalRequired bool     `json:"approval_required,omitempty"`
	ExecutorHint     string   `json:"executor_hint,omitempty"`
	SourcePath       string   `json:"source_path,omitempty"`
	Body             string   `json:"body,omitempty"`
}

type SkillRegistry struct {
	SchemaVersion string            `json:"schema_version"`
	Skills        []SkillDefinition `json:"skills"`
}
