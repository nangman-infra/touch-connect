package tests

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestMessageQualityPolicyDocFieldsMatchContractJSONTags(t *testing.T) {
	assertJSONTags(t, reflect.TypeOf(contracts.PhraseologyPolicy{}), []string{
		"policy_ref",
		"policy_version",
		"scope_kind",
		"scope_ref",
		"applies_to_capabilities",
		"required_fields",
		"readback",
		"constraint_rules",
		"ambiguity_rules",
		"fallback_action",
		"severity",
		"audit_mode",
	})
	assertJSONTags(t, reflect.TypeOf(contracts.CapabilityClaim{}), []string{
		"claim_ref",
		"capabilities",
		"version_constraints",
		"scope",
		"fallback_chain",
		"required_evidence",
	})
	assertJSONTags(t, reflect.TypeOf(contracts.QualityDecision{}), []string{
		"quality_decision_ref",
		"message_ref",
		"policy_ref",
		"policy_version",
		"decision",
		"violations",
		"fallback_action",
		"created_at",
		"created_by",
	})
	assertJSONTags(t, reflect.TypeOf(contracts.QualityViolation{}), []string{
		"code",
		"field",
		"detail",
		"severity",
		"suggested_fix",
	})
}

func assertJSONTags(t *testing.T, typ reflect.Type, expected []string) {
	t.Helper()
	tags := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			tags[name] = true
		}
	}
	for _, name := range expected {
		if !tags[name] {
			t.Fatalf("%s missing json tag %q; tags=%+v", typ.Name(), name, tags)
		}
	}
}
