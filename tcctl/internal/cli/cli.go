package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/nangman-infra/touch-connect/tcctl/internal/config"
	"github.com/nangman-infra/touch-connect/tcctl/internal/controlapi"
)

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	return e.Err.Error()
}

type Runtime struct {
	client *controlapi.Client
	config config.Config
	stdout io.Writer
	stderr io.Writer
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return usageError(err)
	}
	global := flag.NewFlagSet("tcctl", flag.ContinueOnError)
	global.SetOutput(stderr)
	global.StringVar(&cfg.ControlURL, "control-url", cfg.ControlURL, "tc-control base URL")
	global.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "request timeout")
	global.StringVar(&cfg.ExpectedContract, "contract-version", cfg.ExpectedContract, "expected touch-connect contract version")
	global.BoolVar(&cfg.JSON, "json", false, "write JSON output")
	showVersion := global.Bool("version", false, "show tcctl version")
	if err := global.Parse(args); err != nil {
		return usageError(err)
	}
	cfg, err = cfg.Validated()
	if err != nil {
		return usageError(err)
	}
	if *showVersion {
		fmt.Fprintf(stdout, "tcctl %s\n", cfg.Version)
		return nil
	}
	remaining := global.Args()
	if len(remaining) == 0 {
		writeRootHelp(stdout)
		return nil
	}
	runtime := Runtime{
		client: controlapi.New(cfg.ControlURL, cfg.Timeout),
		config: cfg,
		stdout: stdout,
		stderr: stderr,
	}
	if err := runtime.ensureCompatible(ctx, remaining); err != nil {
		return err
	}
	return runtime.dispatch(ctx, remaining)
}

func (r Runtime) ensureCompatible(ctx context.Context, args []string) error {
	if len(args) >= 2 && args[0] == "server" && args[1] == "version" {
		return nil
	}
	version, err := r.client.Version(ctx)
	if err != nil {
		return unavailableError(err)
	}
	if r.config.ExpectedContract != "" && version.ContractVersion != r.config.ExpectedContract {
		return usageError(fmt.Errorf("incompatible contract version: expected %s, got %s", r.config.ExpectedContract, version.ContractVersion))
	}
	return nil
}

func (r Runtime) dispatch(ctx context.Context, args []string) error {
	switch args[0] {
	case "server":
		return r.server(ctx, args[1:])
	case "endpoint":
		return r.endpoint(ctx, args[1:])
	case "message":
		return r.message(ctx, args[1:])
	case "task":
		return r.task(ctx, args[1:])
	case "artifact":
		return r.artifact(ctx, args[1:])
	case "approval":
		return r.approval(ctx, args[1:])
	case "dlq":
		return r.dlq(ctx, args[1:])
	case "scenario":
		return r.scenario(ctx, args[1:])
	case "help", "-h", "--help":
		writeRootHelp(r.stdout)
		return nil
	default:
		return usageError(fmt.Errorf("unknown command %q", args[0]))
	}
}

func commandFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func requireArgs(args []string, count int, usage string) error {
	if len(args) < count {
		return usageError(fmt.Errorf("usage: %s", usage))
	}
	return nil
}

func usageError(err error) error {
	return ExitError{Code: 2, Err: err}
}

func unavailableError(err error) error {
	return ExitError{Code: 3, Err: err}
}

func writeRootHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl [--control-url URL] [--timeout DURATION] [--json] <group> <command>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "groups:")
	fmt.Fprintln(w, "  server      health and version")
	fmt.Fprintln(w, "  endpoint    endpoint list, inspect, capabilities")
	fmt.Fprintln(w, "  message     send, list, inspect, history")
	fmt.Fprintln(w, "  task        status and history by task/correlation ref")
	fmt.Fprintln(w, "  artifact    list and inspect artifact versions")
	fmt.Fprintln(w, "  approval    list and inspect approval records")
	fmt.Fprintln(w, "  dlq         list and inspect dead-letter records")
	fmt.Fprintln(w, "  scenario    run and verify canonical scenario records")
}

func parseTimeout(value string) (time.Duration, error) {
	return time.ParseDuration(value)
}
