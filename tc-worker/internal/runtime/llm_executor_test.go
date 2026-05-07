package runtime

import (
	"strings"
	"testing"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

func TestLLMPromptIncludesTakeoverAndHandoffContext(t *testing.T) {
	prompt := llmPromptFromInput(ExecutionInput{
		MessageRef:         "tc://message/msg_resume",
		AttemptRef:         "tc://attempt/att_resume",
		TargetCapability:   "ai.review",
		CorrelationRef:     "tc://task/resume",
		Takeover:           true,
		LastCheckpointRef:  "tc://checkpoint/previous",
		ResumeSummary:      "previous attempt timed out after partial work",
		ResumeArtifactRefs: []string{"tc://artifact-version/partial"},
		Payload: contracts.Payload{
			Summary: "continue review",
			Body:    "Continue from the previous partial result.",
		},
		Constraints: []contracts.Constraint{
			{Code: "preserve_contract", Summary: "Keep contract markers.", Details: "Do not omit WORKER_RESULT_READY."},
		},
		HandoffContext: HandoffContext{
			TaskRef: "tc://task/resume",
			Messages: []HandoffMessage{
				{
					MessageRef:       "tc://message/prior",
					TargetCapability: "code.change",
					State:            "completed",
					AttemptRef:       "tc://attempt/prior",
					RedeliveryCount:  1,
					Summary:          "prior summary",
					Body:             "prior body",
				},
			},
			Artifacts: []HandoffArtifact{
				{
					ArtifactVersionRef: "tc://artifact-version/partial",
					ArtifactRef:        "tc://artifact/partial",
					MessageRef:         "tc://message/prior",
					AttemptRef:         "tc://attempt/prior",
					Kind:               "log_bundle",
					MediaType:          "application/json",
					StorageRef:         "file:///tmp/partial.json",
					Summary:            "partial work",
					Stdout:             "already changed files A and B",
					Stderr:             "timeout",
					UsedSkillRefs:      []string{"tc://skill/code"},
				},
			},
		},
	})
	for _, want := range []string{
		"takeover: true",
		"last_checkpoint_ref: tc://checkpoint/previous",
		"resume_summary: previous attempt timed out after partial work",
		"prior_messages:",
		"redelivery_count: 1",
		"prior_artifacts:",
		"used_skill_refs: tc://skill/code",
		"already changed files A and B",
		"preserve_contract: Keep contract markers.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to include %q, got %s", want, prompt)
		}
	}
}

func TestOpenAIResponseOutputExtractionAndFailures(t *testing.T) {
	output := outputTextFromOpenAIResponse(openAIResponsesResponse{
		Output: []openAIResponsesItem{
			{Content: []openAIResponsesContent{{Type: "output_text", Text: " first "}}},
			{Content: []openAIResponsesContent{{Type: "ignored", Text: "skip"}, {Type: "output_text", Text: "second"}}},
		},
	})
	if output != "first\nsecond" {
		t.Fatalf("unexpected output extraction: %q", output)
	}
	failed := failedLLMResult("reason", "summary", "stderr", 42)
	if failed.Outcome != ExecutionOutcomeFailed || failed.FailureReasonCode != "reason" || failed.DurationMS != 42 {
		t.Fatalf("unexpected failed result: %+v", failed)
	}
}
