package runtime

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

const (
	maxHandoffMessages             = 20
	maxHandoffArtifacts            = 20
	maxHandoffArtifactContentBytes = 64 * 1024
	maxHandoffPromptFieldChars     = 12 * 1024
)

func handoffContextFromSnapshot(claim contracts.ClaimMessageResponse, snapshot contracts.SnapshotResponse) HandoffContext {
	taskRef := strings.TrimSpace(claim.CorrelationRef)
	referencedMessages, referencedArtifacts := referencedHandoffRefs(claim.Payload.References)
	messageRefs := map[string]struct{}{}
	context := HandoffContext{TaskRef: taskRef}
	for _, message := range snapshot.Messages {
		if message.MessageRef == claim.MessageRef {
			continue
		}
		_, directlyReferenced := referencedMessages[message.MessageRef]
		if !directlyReferenced && !sameHandoffTask(taskRef, message) {
			continue
		}
		if message.State != "completed" {
			continue
		}
		context.Messages = append(context.Messages, HandoffMessage{
			MessageRef:       message.MessageRef,
			TargetCapability: message.TargetCapability,
			State:            message.State,
			AttemptRef:       message.AttemptRef,
			RedeliveryCount:  message.RedeliveryCount,
			Summary:          trimPromptField(message.Payload.Summary),
			Body:             trimPromptField(message.Payload.Body),
		})
		messageRefs[message.MessageRef] = struct{}{}
		if len(context.Messages) >= maxHandoffMessages {
			break
		}
	}
	for _, artifact := range snapshot.Artifacts {
		_, messageMatched := messageRefs[artifact.MessageRef]
		_, artifactMatched := referencedArtifacts[artifact.ArtifactVersionRef]
		if !artifactMatched {
			_, artifactMatched = referencedArtifacts[artifact.ArtifactRef]
		}
		if !messageMatched && !artifactMatched {
			continue
		}
		context.Artifacts = append(context.Artifacts, handoffArtifactFromRecord(artifact))
		if len(context.Artifacts) >= maxHandoffArtifacts {
			break
		}
	}
	if context.TaskRef == "" && len(context.Messages) == 0 && len(context.Artifacts) == 0 {
		return HandoffContext{}
	}
	return context
}

func referencedHandoffRefs(references []contracts.Reference) (map[string]struct{}, map[string]struct{}) {
	messages := map[string]struct{}{}
	artifacts := map[string]struct{}{}
	for _, reference := range references {
		ref := strings.TrimSpace(reference.Ref)
		if ref == "" {
			continue
		}
		switch strings.TrimSpace(reference.Type) {
		case "message", "message_ref":
			messages[ref] = struct{}{}
		case "artifact", "artifact_version", "artifact_version_ref":
			artifacts[ref] = struct{}{}
		default:
			if strings.HasPrefix(ref, "tc://message/") {
				messages[ref] = struct{}{}
			} else if strings.HasPrefix(ref, "tc://artifact/") || strings.HasPrefix(ref, "tc://artifact-version/") {
				artifacts[ref] = struct{}{}
			}
		}
	}
	return messages, artifacts
}

func sameHandoffTask(taskRef string, message contracts.MessageRecord) bool {
	return taskRef != "" && message.CorrelationRef == taskRef
}

func handoffArtifactFromRecord(record contracts.ArtifactRecord) HandoffArtifact {
	artifact := HandoffArtifact{
		ArtifactVersionRef: record.ArtifactVersionRef,
		ArtifactRef:        record.ArtifactRef,
		MessageRef:         record.MessageRef,
		AttemptRef:         record.AttemptRef,
		Kind:               record.Kind,
		MediaType:          record.MediaType,
		StorageRef:         record.StorageRef,
	}
	content := readLocalArtifactContent(record.StorageRef)
	if content == "" {
		return artifact
	}
	var log ExecutionLogArtifact
	if err := json.Unmarshal([]byte(content), &log); err == nil {
		artifact.Summary = trimPromptField(log.Summary)
		artifact.Stdout = trimPromptField(log.Stdout)
		if log.Outcome != ExecutionOutcomeCompleted || log.FailureReasonCode != "" {
			artifact.Stderr = trimPromptField(log.Stderr)
		}
		artifact.UsedSkillRefs = append([]string(nil), log.UsedSkillRefs...)
		return artifact
	}
	artifact.Content = trimPromptField(content)
	return artifact
}

func readLocalArtifactContent(storageRef string) string {
	path, ok := localFilePathFromStorageRef(storageRef)
	if !ok {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > maxHandoffArtifactContentBytes {
		data = data[:maxHandoffArtifactContentBytes]
	}
	return string(data)
}

func localFilePathFromStorageRef(storageRef string) (string, bool) {
	storageRef = strings.TrimSpace(storageRef)
	if storageRef == "" {
		return "", false
	}
	parsed, err := url.Parse(storageRef)
	if err == nil && parsed.Scheme == "file" {
		path := parsed.Path
		if path == "" {
			path = strings.TrimPrefix(storageRef, "file://")
		}
		return path, filepath.IsAbs(path)
	}
	path := strings.TrimPrefix(storageRef, "file://")
	return path, filepath.IsAbs(path) && path != storageRef
}

func trimPromptField(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxHandoffPromptFieldChars {
		return value
	}
	return value[:maxHandoffPromptFieldChars] + "\n[truncated]"
}
