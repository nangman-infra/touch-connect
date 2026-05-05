package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
	skillpkg "github.com/nangman-infra/touch-connect/internal/communication/skills"
	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

func (r Runtime) skill(_ context.Context, args []string) error {
	if helpOnly(args) {
		writeSkillHelp(r.stdout)
		return nil
	}
	if args[0] == "help" {
		return r.skillCommandHelp(args[1:])
	}
	if err := requireArgs(args, 1, "tcctl skill <register|list|inspect>"); err != nil {
		return err
	}
	switch args[0] {
	case "register":
		return r.registerSkill(args[1:])
	case "list":
		return r.listSkills(args[1:])
	case "inspect":
		return r.inspectSkill(args[1:])
	default:
		return usageError(fmt.Errorf("unknown skill command %q", args[0]))
	}
}

func (r Runtime) registerSkill(args []string) error {
	if helpOnly(args) {
		flags := commandFlagSet("skill register <absolute_skill_md_path> [flags]", r.stderr)
		flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
		flags.Usage()
		return errHelpRequested
	}
	if err := requireArgs(args, 1, "tcctl skill register <absolute_skill_md_path>"); err != nil {
		return err
	}
	flags := commandFlagSet("skill register <absolute_skill_md_path> [flags]", r.stderr)
	registryPath := flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
	if err := parseCommandFlags(flags, args[1:]); err != nil {
		return err
	}
	if !filepath.IsAbs(args[0]) {
		return usageError(errors.New("skill path must be absolute"))
	}
	if !filepath.IsAbs(*registryPath) {
		return usageError(errors.New("registry path must be absolute"))
	}
	skill, err := skillpkg.LoadFile(args[0])
	if err != nil {
		return commandError(err)
	}
	registry, err := skillpkg.LoadRegistry(*registryPath)
	if err != nil {
		return commandError(err)
	}
	registry, err = skillpkg.Upsert(registry, skill)
	if err != nil {
		return commandError(err)
	}
	if err := skillpkg.SaveRegistry(*registryPath, registry); err != nil {
		return commandError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, skill)
	}
	writeSkill(r.stdout, skill)
	return nil
}

func (r Runtime) listSkills(args []string) error {
	flags := commandFlagSet("skill list [flags]", r.stderr)
	registryPath := flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	registry, err := skillpkg.LoadRegistry(*registryPath)
	if err != nil {
		return commandError(err)
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, registry.Skills)
	}
	for _, skill := range registry.Skills {
		fmt.Fprintf(r.stdout, "%s\t%s\t%s\t%s\n", skill.SkillRef, skill.Kind, strings.Join(skill.Capabilities, ","), skill.Name)
	}
	return nil
}

func (r Runtime) inspectSkill(args []string) error {
	if err := requireArgs(args, 1, "tcctl skill inspect <skill_ref_or_name>"); err != nil {
		return err
	}
	flags := commandFlagSet("skill inspect <skill_ref_or_name> [flags]", r.stderr)
	registryPath := flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
	if err := parseCommandFlags(flags, args[1:]); err != nil {
		return err
	}
	registry, err := skillpkg.LoadRegistry(*registryPath)
	if err != nil {
		return commandError(err)
	}
	skill, ok := skillpkg.Find(registry, args[0])
	if !ok {
		return commandError(errors.New("skill not found"))
	}
	if r.config.JSON {
		return output.WriteJSON(r.stdout, skill)
	}
	writeSkill(r.stdout, skill)
	return nil
}

func writeSkillHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl skill <register|list|inspect>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  register  parse and register a local SKILL.md")
	fmt.Fprintln(w, "  list      list registered local skills")
	fmt.Fprintln(w, "  inspect   inspect one registered local skill")
}

func (r Runtime) skillCommandHelp(args []string) error {
	if len(args) == 0 {
		writeSkillHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "register":
		return r.registerSkill([]string{"-h"})
	case "list":
		flags := commandFlagSet("skill list [flags]", r.stderr)
		flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
		flags.Usage()
		return errHelpRequested
	case "inspect":
		flags := commandFlagSet("skill inspect <skill_ref_or_name> [flags]", r.stderr)
		flags.String("registry", defaultSkillRegistryPath(), "local skill registry path")
		flags.Usage()
		return errHelpRequested
	default:
		return usageError(fmt.Errorf("unknown skill help topic %q", args[0]))
	}
}

func writeSkill(w io.Writer, skill contracts.SkillDefinition) {
	fmt.Fprintf(w, "skill=%s\nname=%s\nkind=%s\ncapabilities=%s\nsource=%s\n",
		skill.SkillRef,
		skill.Name,
		skill.Kind,
		strings.Join(skill.Capabilities, ","),
		skill.SourcePath,
	)
}

func defaultSkillRegistryPath() string {
	if value := strings.TrimSpace(os.Getenv("TCCTL_SKILL_REGISTRY")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("TC_SKILL_REGISTRY")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/tmp/touch-connect-skill-registry.json"
	}
	return home + "/.touch-connect/skills/registry.json"
}
