package skills

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func LoadFile(path string) (contracts.SkillDefinition, error) {
	if !filepath.IsAbs(path) {
		return contracts.SkillDefinition{}, errors.New("skill path must be absolute")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return contracts.SkillDefinition{}, err
	}
	skill, err := ParseMarkdown(path, string(body))
	if err != nil {
		return contracts.SkillDefinition{}, err
	}
	return Validate(skill)
}

func LoadDir(dir string) ([]contracts.SkillDefinition, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("skills dir must be absolute")
	}
	var loaded []contracts.SkillDefinition
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.ToLower(entry.Name()) != "skill.md" {
			return nil
		}
		skill, err := LoadFile(path)
		if err != nil {
			return err
		}
		loaded = append(loaded, skill)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return loaded, nil
}

func ParseMarkdown(path string, content string) (contracts.SkillDefinition, error) {
	metadata, body := splitFrontMatter(content)
	skill := contracts.SkillDefinition{
		SkillRef:   metadata.scalar("skill_ref"),
		Name:       metadata.scalar("name"),
		Kind:       metadata.scalar("kind"),
		Body:       strings.TrimSpace(body),
		SourcePath: path,
	}
	skill.Description = metadata.scalar("description")
	skill.Capabilities = metadata.list("capabilities")
	skill.AppliesTo = metadata.list("applies_to")
	skill.ExecutorHint = metadata.scalar("executor_hint")
	skill.ApprovalRequired = metadata.bool("approval_required")
	inferSkillDefaults(&skill, path)
	return skill, nil
}

func MatchCapability(skills []contracts.SkillDefinition, capability string) (contracts.SkillDefinition, bool) {
	for _, skill := range skills {
		if slices.Contains(skill.Capabilities, capability) {
			return skill, true
		}
	}
	return contracts.SkillDefinition{}, false
}

func Validate(s contracts.SkillDefinition) (contracts.SkillDefinition, error) {
	return validateSkillDefinition(s)
}

type frontMatter map[string][]string

func splitFrontMatter(content string) (frontMatter, string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return frontMatter{}, normalized
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	index := strings.Index(rest, "\n---")
	if index < 0 {
		return frontMatter{}, normalized
	}
	raw := rest[:index]
	body := strings.TrimPrefix(rest[index:], "\n---")
	body = strings.TrimPrefix(body, "\n")
	return parseFrontMatter(raw), body
}

func parseFrontMatter(raw string) frontMatter {
	values := frontMatter{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	currentKey := ""
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && currentKey != "" {
			values[currentKey] = append(values[currentKey], cleanScalar(strings.TrimPrefix(trimmed, "- ")))
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		currentKey = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if value == "" {
			values[currentKey] = nil
			continue
		}
		values[currentKey] = parseValueList(value)
	}
	return values
}

func (m frontMatter) scalar(key string) string {
	values := m[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (m frontMatter) list(key string) []string {
	values := m[key]
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func (m frontMatter) bool(key string) bool {
	switch strings.ToLower(m.scalar(key)) {
	case "true", "yes", "y", "1":
		return true
	default:
		return false
	}
}

func parseValueList(value string) []string {
	value = cleanScalar(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		parts := strings.Split(value, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			item := cleanScalar(part)
			if item != "" {
				items = append(items, item)
			}
		}
		return items
	}
	return []string{value}
}

func cleanScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'")
	return strings.TrimSpace(value)
}

func inferSkillDefaults(skill *contracts.SkillDefinition, path string) {
	base := strings.TrimSuffix(filepath.Base(filepath.Dir(path)), filepath.Ext(filepath.Base(filepath.Dir(path))))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = strings.TrimSuffix(filepath.Base(path), filepath.Ext(filepath.Base(path)))
	}
	if skill.Name == "" {
		skill.Name = titleFromSlug(base)
	}
	if skill.SkillRef == "" {
		skill.SkillRef = "tc://skill/" + safeRefPart(base)
	}
	if skill.Kind == "" {
		skill.Kind = contracts.SkillKindGuidance
	}
}

func validateSkillDefinition(skill contracts.SkillDefinition) (contracts.SkillDefinition, error) {
	if strings.TrimSpace(skill.SkillRef) == "" {
		return contracts.SkillDefinition{}, errors.New("skill_ref is required")
	}
	if !strings.HasPrefix(skill.SkillRef, "tc://skill/") {
		return contracts.SkillDefinition{}, errors.New("skill_ref must start with tc://skill/")
	}
	if strings.TrimSpace(skill.Name) == "" {
		return contracts.SkillDefinition{}, errors.New("skill name is required")
	}
	switch skill.Kind {
	case contracts.SkillKindGuidance, contracts.SkillKindExecutable:
	default:
		return contracts.SkillDefinition{}, errors.New("skill kind must be guidance or executable")
	}
	skill.Body = strings.TrimSpace(skill.Body)
	if skill.Body == "" {
		return contracts.SkillDefinition{}, errors.New("skill body is required")
	}
	return skill, nil
}

func titleFromSlug(value string) string {
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ReplaceAll(value, "_", " ")
	return strings.TrimSpace(value)
}

func safeRefPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, item := range value {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			builder.WriteRune(item)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "local"
	}
	return result
}
