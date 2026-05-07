package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type managerOptions struct {
	TaskRef              string
	Capability           string
	Summary              string
	Body                 string
	BodyFile             string
	Sender               string
	MessageRef           string
	TargetEndpointRef    string
	PreferredEndpointRef string
	DependsOn            string
	QualityGate          contracts.QualityGateMode
	ReadbackRequired     bool
	Send                 bool
	Watch                bool
	Once                 bool
	Interval             time.Duration
}

func (r Runtime) manager(ctx context.Context, args []string) error {
	flags := commandFlagSet("manager [flags]", r.stderr)
	options := managerOptions{}
	flags.StringVar(&options.TaskRef, "task", "", "task/correlation ref to focus")
	flags.StringVar(&options.Capability, "capability", "", "target capability for --send or cockpit filtering")
	flags.StringVar(&options.Summary, "summary", "", "payload summary for --send")
	flags.StringVar(&options.Body, "body", "", "payload body for --send")
	flags.StringVar(&options.BodyFile, "body-file", "", "read payload body for --send from file")
	flags.StringVar(&options.Sender, "sender", "tc://endpoint/tcctl", "sender endpoint ref for --send")
	flags.StringVar(&options.MessageRef, "message-ref", "", "optional message ref for --send")
	flags.StringVar(&options.TargetEndpointRef, "target-endpoint", "", "route --send only to this endpoint ref")
	flags.StringVar(&options.PreferredEndpointRef, "prefer-endpoint", "", "prefer this endpoint ref for --send, with fallback")
	flags.StringVar(&options.DependsOn, "depends-on", "", "comma-separated message refs that must complete before --send can be claimed")
	qualityGate := flags.String("quality-gate", contracts.QualityGateWarn.String(), "quality gate mode for --send: enforce, warn, or skip")
	flags.BoolVar(&options.ReadbackRequired, "readback-required", true, "require worker readback for --send")
	flags.BoolVar(&options.Send, "send", false, "send a manager handoff before rendering the cockpit")
	flags.BoolVar(&options.Watch, "watch", false, "keep refreshing the cockpit")
	flags.BoolVar(&options.Once, "once", false, "print one cockpit frame and exit")
	flags.DurationVar(&options.Interval, "interval", time.Second, "watch refresh interval")
	if err := parseCommandFlags(flags, args); err != nil {
		return err
	}
	gate, err := contracts.ParseQualityGateMode(*qualityGate)
	if err != nil {
		return usageError(err)
	}
	options.QualityGate = gate
	if options.Interval <= 0 {
		return usageError(fmt.Errorf("--interval must be positive"))
	}
	if !options.Send && options.Summary != "" {
		return usageError(fmt.Errorf("--summary requires --send"))
	}
	if !options.Send && (options.Body != "" || options.BodyFile != "") {
		return usageError(fmt.Errorf("--body and --body-file require --send"))
	}
	if options.Send {
		res, err := r.managerSend(ctx, &options)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.stdout, "sent message=%s state=%s delivery=%s task=%s\n", res.MessageRef, res.State, res.DeliveryRef, options.TaskRef)
		if !options.Watch && !options.Once {
			options.Once = true
		}
	}
	if !options.Watch {
		options.Once = true
	}
	for {
		snapshot, err := r.client.Snapshot(ctx)
		if err != nil {
			return unavailableError(err)
		}
		writeManagerCockpit(r.stdout, snapshot, options)
		if options.Once {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(options.Interval):
		}
	}
}

func (r Runtime) managerSend(ctx context.Context, options *managerOptions) (contracts.MessageIngressResponse, error) {
	body, err := resolvePayloadBody(options.Body, options.BodyFile)
	if err != nil {
		return contracts.MessageIngressResponse{}, usageError(err)
	}
	if options.TaskRef == "" {
		options.TaskRef = "tc://task/manager_" + time.Now().UTC().Format("20060102_150405")
	}
	if options.Capability == "" || options.Summary == "" || body == "" {
		return contracts.MessageIngressResponse{}, usageError(fmt.Errorf("--send requires --capability, --summary, and one of --body or --body-file"))
	}
	req := contracts.MessageIngressRequest{
		MessageRef:           options.MessageRef,
		SenderEndpointRef:    options.Sender,
		TargetCapability:     options.Capability,
		TargetEndpointRef:    options.TargetEndpointRef,
		PreferredEndpointRef: options.PreferredEndpointRef,
		DependsOnMessageRefs: splitCSV(options.DependsOn),
		CorrelationRef:       options.TaskRef,
		ReadbackRequired:     options.ReadbackRequired,
		QualityGate:          options.QualityGate,
		Constraints:          []contracts.Constraint{},
		Payload: contracts.Payload{
			Summary:    options.Summary,
			Body:       body,
			References: []contracts.Reference{},
		},
	}
	value, err := r.client.SendMessage(ctx, req)
	if err != nil {
		return contracts.MessageIngressResponse{}, messageSendError(err, r.stderr)
	}
	return value, nil
}

func writeManagerCockpit(w io.Writer, snapshot contracts.SnapshotResponse, options managerOptions) {
	fmt.Fprintf(w, "\ntouch-connect manager generated=%s source=%s\n", snapshot.Freshness.GeneratedAt, snapshot.Freshness.Source)
	writeManagerSystem(w, snapshot, options)
	writeManagerWorkers(w, snapshot.Endpoints, options)
	writeManagerTasks(w, snapshot.Messages, options)
	writeManagerTimeline(w, snapshot, options)
	writeManagerNextActions(w, options)
}

func writeManagerSystem(w io.Writer, snapshot contracts.SnapshotResponse, options managerOptions) {
	workersOnline := 0
	for _, endpoint := range snapshot.Endpoints {
		if endpoint.ConnectionState == "online" {
			workersOnline++
		}
	}
	messageCounts := messageStateCounts(snapshot.Messages)
	qualityCounts := qualityDecisionCounts(snapshot.QualityDecisions)
	taskLabel := "all"
	if options.TaskRef != "" {
		taskLabel = options.TaskRef
	}
	fmt.Fprintf(w, "System task=%s workers=%d/%d online messages=%d accepted=%d claimed=%d completed=%d failed=%d quality_passed=%d warned=%d rejected=%d artifacts=%d\n",
		taskLabel,
		workersOnline,
		len(snapshot.Endpoints),
		len(snapshot.Messages),
		messageCounts["accepted"],
		messageCounts["claimed"],
		messageCounts["completed"],
		messageCounts["failed"],
		qualityCounts[contracts.QualityDecisionPassed],
		qualityCounts[contracts.QualityDecisionWarned],
		qualityCounts[contracts.QualityDecisionRejected],
		len(snapshot.Artifacts),
	)
}

func writeManagerWorkers(w io.Writer, endpoints []contracts.EndpointRecord, options managerOptions) {
	fmt.Fprintln(w, "\nWorkers")
	for _, endpoint := range sortedEndpoints(endpoints) {
		caps := endpointCapabilityNames(endpoint)
		if options.Capability != "" && !containsString(caps, options.Capability) {
			continue
		}
		progress := endpoint.ProgressSummary
		if endpoint.CurrentAttemptRef != "" {
			progress = shortRef(endpoint.CurrentAttemptRef) + " " + progress
		}
		fmt.Fprintf(w, "  %-8s %-32s caps=%s hints=%s progress=%s\n",
			endpoint.ConnectionState,
			shortRef(endpoint.EndpointRef),
			compact(strings.Join(caps, ","), 48),
			compact(strings.Join(endpoint.ExecutionHints, ","), 32),
			compact(progress, 40),
		)
	}
}

func writeManagerTasks(w io.Writer, messages []contracts.MessageRecord, options managerOptions) {
	type taskSummary struct {
		ref       string
		total     int
		active    int
		completed int
		failed    int
		latest    string
	}
	tasks := map[string]taskSummary{}
	for _, message := range messages {
		if !managerMessageMatches(message, options) {
			continue
		}
		ref := defaultString(message.CorrelationRef, "-")
		summary := tasks[ref]
		summary.ref = ref
		summary.total++
		switch message.State {
		case "completed":
			summary.completed++
		case "failed", "dead_lettered", "blocked":
			summary.failed++
		default:
			summary.active++
		}
		if message.MessageRef > summary.latest {
			summary.latest = message.Payload.Summary
		}
		tasks[ref] = summary
	}
	refs := make([]string, 0, len(tasks))
	for ref := range tasks {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	fmt.Fprintln(w, "\nTasks")
	if len(refs) == 0 {
		fmt.Fprintln(w, "  no matching tasks")
		return
	}
	start := maxInt(0, len(refs)-8)
	for _, ref := range refs[start:] {
		task := tasks[ref]
		fmt.Fprintf(w, "  %-32s total=%d active=%d completed=%d failed=%d latest=%q\n",
			compact(shortRef(task.ref), 32),
			task.total,
			task.active,
			task.completed,
			task.failed,
			compact(task.latest, 52),
		)
	}
}

func writeManagerTimeline(w io.Writer, snapshot contracts.SnapshotResponse, options managerOptions) {
	events := managerTimeline(snapshot, options)
	fmt.Fprintln(w, "\nTimeline")
	if len(events) == 0 {
		fmt.Fprintln(w, "  no matching activity")
		return
	}
	start := maxInt(0, len(events)-12)
	for _, event := range events[start:] {
		fmt.Fprintln(w, "  "+event)
	}
}

func managerTimeline(snapshot contracts.SnapshotResponse, options managerOptions) []string {
	var events []string
	messages := make([]contracts.MessageRecord, 0)
	for _, message := range sortedMessages(snapshot.Messages) {
		if !managerMessageMatches(message, options) {
			continue
		}
		messages = append(messages, message)
	}
	if len(messages) > 4 {
		messages = messages[len(messages)-4:]
	}
	for _, message := range messages {
		events = append(events, fmt.Sprintf("message %-10s %s cap=%s task=%s %q",
			message.State,
			shortRef(message.MessageRef),
			message.TargetCapability,
			shortRef(message.CorrelationRef),
			compact(message.Payload.Summary, 72),
		))
		attemptRefs := map[string]struct{}{}
		for _, attempt := range sortedAttempts(snapshot.Attempts) {
			if attempt.MessageRef != message.MessageRef {
				continue
			}
			attemptRefs[attempt.AttemptRef] = struct{}{}
			events = append(events, fmt.Sprintf("attempt %-10s %s worker=%s epoch=%d",
				attempt.State,
				shortRef(attempt.AttemptRef),
				shortRef(attempt.EndpointRef),
				attempt.ClaimEpoch,
			))
		}
		for _, readback := range sortedReadbacks(snapshot.Readbacks) {
			if _, ok := attemptRefs[readback.AttemptRef]; !ok {
				continue
			}
			events = append(events, fmt.Sprintf("readback %-10s %s worker=%s %q",
				"recorded",
				shortRef(readback.ReadbackRef),
				shortRef(readback.EndpointRef),
				compact(readback.Understanding, 72),
			))
		}
		for _, checkpoint := range sortedCheckpoints(snapshot.Checkpoints) {
			if _, ok := attemptRefs[checkpoint.AttemptRef]; !ok {
				continue
			}
			events = append(events, fmt.Sprintf("checkpoint %-10s %s artifacts=%d %q",
				checkpoint.State,
				shortRef(checkpoint.CheckpointRef),
				len(checkpoint.ArtifactRefs),
				compact(checkpoint.Summary, 72),
			))
		}
		for _, artifact := range sortedArtifacts(snapshot.Artifacts) {
			if artifact.MessageRef != message.MessageRef {
				continue
			}
			events = append(events, fmt.Sprintf("artifact %-10s %s kind=%s",
				"written",
				shortRef(artifact.ArtifactVersionRef),
				artifact.Kind,
			))
		}
	}
	return events
}

func writeManagerNextActions(w io.Writer, options managerOptions) {
	fmt.Fprintln(w, "\nNext")
	task := options.TaskRef
	if task == "" {
		task = "<task_ref>"
	}
	fmt.Fprintf(w, "  watch:   tcctl manager --task %s --watch\n", task)
	fmt.Fprintf(w, "  history: tcctl task history %s\n", task)
	fmt.Fprintln(w, "  send:    tcctl manager --send --capability <cap> --summary <text> --body-file /absolute/path/body.md")
}

func managerMessageMatches(message contracts.MessageRecord, options managerOptions) bool {
	if options.TaskRef != "" && message.CorrelationRef != options.TaskRef {
		return false
	}
	if options.Capability != "" && message.TargetCapability != options.Capability {
		return false
	}
	return true
}

func messageStateCounts(messages []contracts.MessageRecord) map[string]int {
	counts := map[string]int{}
	for _, message := range messages {
		counts[message.State]++
	}
	return counts
}

func qualityDecisionCounts(decisions []contracts.QualityDecision) map[string]int {
	counts := map[string]int{}
	for _, decision := range decisions {
		counts[decision.Decision]++
	}
	return counts
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func shortRef(ref string) string {
	if index := strings.LastIndex(ref, "/"); index >= 0 && index+1 < len(ref) {
		return ref[index+1:]
	}
	return ref
}

func compact(value string, maxWidth int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxWidth <= 3 || len([]rune(value)) <= maxWidth {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxWidth-3]) + "..."
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
