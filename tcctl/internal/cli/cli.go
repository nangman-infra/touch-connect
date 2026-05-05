package cli

import (
	"context"
	"errors"
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

var errHelpRequested = errors.New("help requested")

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
		if flagHelpRequested(err) {
			writeRootHelp(stdout)
			return nil
		}
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
	if !isHelpRequest(remaining) {
		if err := runtime.ensureCompatible(ctx, remaining); err != nil {
			return err
		}
	}
	if err := runtime.dispatch(ctx, remaining); helpWasShown(err) {
		return nil
	} else if err != nil {
		return err
	}
	return nil
}

func (r Runtime) ensureCompatible(ctx context.Context, args []string) error {
	if len(args) >= 2 && args[0] == "server" && args[1] == "version" {
		return nil
	}
	if len(args) >= 1 && args[0] == "skill" {
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
	case "help":
		return r.help(args[1:])
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
	case "skill":
		return r.skill(ctx, args[1:])
	case "manager":
		return r.manager(ctx, args[1:])
	case "monitor":
		return r.monitor(ctx, args[1:])
	case "scenario":
		return r.scenario(ctx, args[1:])
	case "-h", "--help":
		writeRootHelp(r.stdout)
		return nil
	default:
		return usageError(fmt.Errorf("unknown command %q", args[0]))
	}
}

func commandFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "usage: tcctl %s\n", name)
		if hasVisibleFlags(flags) {
			fmt.Fprintln(flags.Output(), "")
			fmt.Fprintln(flags.Output(), "flags:")
			flags.PrintDefaults()
		}
	}
	return flags
}

func parseCommandFlags(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		if flagHelpRequested(err) {
			return errHelpRequested
		}
		return usageError(err)
	}
	return nil
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

func commandError(err error) error {
	return ExitError{Code: 1, Err: err}
}

func writeRootHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl [--control-url URL] [--timeout DURATION] [--json] <group|command> [command]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "groups:")
	fmt.Fprintln(w, "  server      health and version")
	fmt.Fprintln(w, "  endpoint    endpoint list, inspect, capabilities")
	fmt.Fprintln(w, "  message     send, list, inspect, history, and tail")
	fmt.Fprintln(w, "  task        status, history, and watch by task/correlation ref")
	fmt.Fprintln(w, "  artifact    list, inspect, lineage, and finalize artifact versions")
	fmt.Fprintln(w, "  approval    list, inspect, chain, approve, and reject approval records")
	fmt.Fprintln(w, "  dlq         list and inspect dead-letter records")
	fmt.Fprintln(w, "  skill       register, list, and inspect local AI skills")
	fmt.Fprintln(w, "  manager     send, watch, and diagnose handoffs from one operator cockpit")
	fmt.Fprintln(w, "  monitor     one-screen standalone operator view")
	fmt.Fprintln(w, "  scenario    run and verify canonical scenario records")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "use \"tcctl help <group>\" or \"tcctl <group> <command> -h\" for command help")
}

func parseTimeout(value string) (time.Duration, error) {
	return time.ParseDuration(value)
}

func (r Runtime) help(args []string) error {
	if len(args) == 0 {
		writeRootHelp(r.stdout)
		return nil
	}
	switch args[0] {
	case "server":
		return r.server(context.Background(), append([]string{"help"}, args[1:]...))
	case "endpoint":
		return r.endpoint(context.Background(), append([]string{"help"}, args[1:]...))
	case "message":
		return r.message(context.Background(), append([]string{"help"}, args[1:]...))
	case "task":
		return r.task(context.Background(), append([]string{"help"}, args[1:]...))
	case "artifact":
		return r.artifact(context.Background(), append([]string{"help"}, args[1:]...))
	case "approval":
		return r.approval(context.Background(), append([]string{"help"}, args[1:]...))
	case "dlq":
		return r.dlq(context.Background(), append([]string{"help"}, args[1:]...))
	case "skill":
		return r.skill(context.Background(), append([]string{"help"}, args[1:]...))
	case "manager":
		return r.manager(context.Background(), append(args[1:], "-h"))
	case "monitor":
		return r.monitor(context.Background(), append(args[1:], "-h"))
	case "scenario":
		return r.scenario(context.Background(), append([]string{"help"}, args[1:]...))
	default:
		return usageError(fmt.Errorf("unknown help topic %q", args[0]))
	}
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		return true
	}
	for _, arg := range args[1:] {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func helpOnly(args []string) bool {
	return len(args) == 0 || args[0] == "-h" || args[0] == "--help"
}

func flagHelpRequested(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

func helpWasShown(err error) bool {
	return errors.Is(err, errHelpRequested)
}

func hasVisibleFlags(flags *flag.FlagSet) bool {
	hasFlags := false
	flags.VisitAll(func(*flag.Flag) {
		hasFlags = true
	})
	return hasFlags
}
