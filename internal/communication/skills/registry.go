package skills

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const RegistrySchemaVersion = "2026-05-05"

func LoadRegistry(path string) (contracts.SkillRegistry, error) {
	if !filepath.IsAbs(path) {
		return contracts.SkillRegistry{}, errors.New("skill registry path must be absolute")
	}
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return contracts.SkillRegistry{SchemaVersion: RegistrySchemaVersion, Skills: []contracts.SkillDefinition{}}, nil
	}
	if err != nil {
		return contracts.SkillRegistry{}, err
	}
	var registry contracts.SkillRegistry
	if err := json.Unmarshal(body, &registry); err != nil {
		return contracts.SkillRegistry{}, err
	}
	if registry.SchemaVersion == "" {
		registry.SchemaVersion = RegistrySchemaVersion
	}
	for index := range registry.Skills {
		skill, err := Validate(registry.Skills[index])
		if err != nil {
			return contracts.SkillRegistry{}, err
		}
		registry.Skills[index] = skill
	}
	return registry, nil
}

func SaveRegistry(path string, registry contracts.SkillRegistry) error {
	if !filepath.IsAbs(path) {
		return errors.New("skill registry path must be absolute")
	}
	if registry.SchemaVersion == "" {
		registry.SchemaVersion = RegistrySchemaVersion
	}
	for index := range registry.Skills {
		skill, err := Validate(registry.Skills[index])
		if err != nil {
			return err
		}
		registry.Skills[index] = skill
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

func Upsert(registry contracts.SkillRegistry, skill contracts.SkillDefinition) (contracts.SkillRegistry, error) {
	accepted, err := Validate(skill)
	if err != nil {
		return contracts.SkillRegistry{}, err
	}
	if registry.SchemaVersion == "" {
		registry.SchemaVersion = RegistrySchemaVersion
	}
	for index, existing := range registry.Skills {
		if existing.SkillRef == accepted.SkillRef {
			registry.Skills[index] = accepted
			return registry, nil
		}
	}
	registry.Skills = append(registry.Skills, accepted)
	slices.SortFunc(registry.Skills, func(left contracts.SkillDefinition, right contracts.SkillDefinition) int {
		if left.SkillRef < right.SkillRef {
			return -1
		}
		if left.SkillRef > right.SkillRef {
			return 1
		}
		return 0
	})
	return registry, nil
}

func Find(registry contracts.SkillRegistry, ref string) (contracts.SkillDefinition, bool) {
	for _, skill := range registry.Skills {
		if skill.SkillRef == ref || skill.Name == ref {
			return skill, true
		}
	}
	return contracts.SkillDefinition{}, false
}
