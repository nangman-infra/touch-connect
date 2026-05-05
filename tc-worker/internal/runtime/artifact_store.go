package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type ExecutionArtifactStore interface {
	StoreExecutionLog(context.Context, ExecutionInput, ExecutionResult) (contracts.ArtifactVersionRequest, error)
}

type LocalArtifactStoreOptions struct {
	Dir            string
	RoomRef        string
	TaskRef        string
	TaskRevision   int
	RetentionClass string
	AccessScope    string
}

type LocalArtifactStore struct {
	dir            string
	roomRef        string
	taskRef        string
	taskRevision   int
	retentionClass string
	accessScope    string
}

type ExecutionLogArtifact struct {
	MessageRef        string   `json:"message_ref"`
	AttemptRef        string   `json:"attempt_ref"`
	TargetCapability  string   `json:"target_capability"`
	UsedSkillRefs     []string `json:"used_skill_refs,omitempty"`
	Outcome           string   `json:"outcome"`
	Summary           string   `json:"summary"`
	FailureReasonCode string   `json:"failure_reason_code,omitempty"`
	Stdout            string   `json:"stdout,omitempty"`
	Stderr            string   `json:"stderr,omitempty"`
	ExitCode          int      `json:"exit_code"`
	DurationMS        int64    `json:"duration_ms"`
}

func NewLocalArtifactStore(options LocalArtifactStoreOptions) (*LocalArtifactStore, error) {
	accepted, err := options.validated()
	if err != nil {
		return nil, err
	}
	return &LocalArtifactStore{
		dir:            accepted.Dir,
		roomRef:        accepted.RoomRef,
		taskRef:        accepted.TaskRef,
		taskRevision:   accepted.TaskRevision,
		retentionClass: accepted.RetentionClass,
		accessScope:    accepted.AccessScope,
	}, nil
}

func (s *LocalArtifactStore) StoreExecutionLog(_ context.Context, input ExecutionInput, result ExecutionResult) (contracts.ArtifactVersionRequest, error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return contracts.ArtifactVersionRequest{}, err
	}
	body, err := json.MarshalIndent(executionLogFromResult(input, result), "", "  ")
	if err != nil {
		return contracts.ArtifactVersionRequest{}, err
	}
	body = append(body, '\n')
	path := filepath.Join(s.dir, executionLogFileName(input))
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return contracts.ArtifactVersionRequest{}, err
	}
	sum := sha256.Sum256(body)
	artifactRef, artifactVersionRef := executionLogRefs(input)
	return contracts.ArtifactVersionRequest{
		ArtifactRef:        artifactRef,
		ArtifactVersionRef: artifactVersionRef,
		RoomRef:            s.roomRef,
		TaskRef:            s.taskRefFor(input),
		TaskRevision:       s.taskRevision,
		Kind:               "log_bundle",
		MediaType:          "application/json",
		SizeBytes:          int64(len(body)),
		Checksum:           "sha256:" + hex.EncodeToString(sum[:]),
		StorageRef:         "file://" + path,
		RetentionClass:     s.retentionClass,
		AccessScope:        s.accessScope,
		BasedOnMessageRefs: []string{input.MessageRef},
	}, nil
}

func (s *LocalArtifactStore) taskRefFor(input ExecutionInput) string {
	if s.taskRef != "" {
		return s.taskRef
	}
	return "tc://task/execution_" + shortDigest(input.MessageRef)
}

func (o LocalArtifactStoreOptions) validated() (LocalArtifactStoreOptions, error) {
	if o.Dir == "" || !filepath.IsAbs(o.Dir) {
		return LocalArtifactStoreOptions{}, errors.New("artifact dir must be an absolute path")
	}
	if o.RoomRef == "" {
		o.RoomRef = "tc://room/worker-execution"
	}
	if o.TaskRevision == 0 {
		o.TaskRevision = 1
	}
	if o.TaskRevision < 0 {
		return LocalArtifactStoreOptions{}, errors.New("artifact task revision must not be negative")
	}
	if o.RetentionClass == "" {
		o.RetentionClass = "operational"
	}
	if o.AccessScope == "" {
		o.AccessScope = "task"
	}
	return o, nil
}

func executionLogFromResult(input ExecutionInput, result ExecutionResult) ExecutionLogArtifact {
	return ExecutionLogArtifact{
		MessageRef:        input.MessageRef,
		AttemptRef:        input.AttemptRef,
		TargetCapability:  input.TargetCapability,
		UsedSkillRefs:     append([]string(nil), result.UsedSkillRefs...),
		Outcome:           result.Outcome,
		Summary:           result.Summary,
		FailureReasonCode: result.FailureReasonCode,
		Stdout:            result.Stdout,
		Stderr:            result.Stderr,
		ExitCode:          result.ExitCode,
		DurationMS:        result.DurationMS,
	}
}

func executionLogFileName(input ExecutionInput) string {
	return safePathPart(input.MessageRef) + "__" + safePathPart(input.AttemptRef) + "__execution-log.json"
}

func executionLogRefs(input ExecutionInput) (string, string) {
	messageDigest := shortDigest(input.MessageRef)
	versionDigest := shortDigest(input.MessageRef + "|" + input.AttemptRef)
	return "tc://artifact/execution-log_" + messageDigest,
		"tc://artifact-version/execution-log_" + versionDigest
}

func safePathPart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' {
			builder.WriteRune(item)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "ref_" + strconv.Itoa(len(value))
	}
	return builder.String()
}

func shortDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}
