package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/nangman-infra/touch-connect/tcctl/internal/output"
)

func (r Runtime) server(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeServerHelp(r.stdout)
		return nil
	}
	if err := requireArgs(args, 1, "tcctl server <health|version>"); err != nil {
		return err
	}
	switch args[0] {
	case "health":
		value, err := r.client.Health(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteHealth(r.stdout, value)
	case "version":
		value, err := r.client.Version(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteVersion(r.stdout, value)
	default:
		return usageError(fmt.Errorf("unknown server command %q", args[0]))
	}
	return nil
}

func (r Runtime) endpoint(ctx context.Context, args []string) error {
	if helpOnly(args) {
		writeEndpointHelp(r.stdout)
		return nil
	}
	if err := requireArgs(args, 1, "tcctl endpoint <list|inspect|capabilities>"); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		value, err := r.client.Endpoints(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteEndpoints(r.stdout, value)
	case "inspect":
		if err := requireArgs(args[1:], 1, "tcctl endpoint inspect <endpoint_ref>"); err != nil {
			return err
		}
		value, err := r.client.Endpoint(ctx, args[1])
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteEndpoint(r.stdout, value)
	case "capabilities":
		value, err := r.client.Capabilities(ctx)
		if err != nil {
			return unavailableError(err)
		}
		if r.config.JSON {
			return output.WriteJSON(r.stdout, value)
		}
		output.WriteCapabilities(r.stdout, value)
	default:
		return usageError(fmt.Errorf("unknown endpoint command %q", args[0]))
	}
	return nil
}

func writeServerHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl server <health|version>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  health     show tc-control health")
	fmt.Fprintln(w, "  version    show tc-control and contract versions")
}

func writeEndpointHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: tcctl endpoint <list|inspect|capabilities>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  list                         list registered endpoints")
	fmt.Fprintln(w, "  inspect <endpoint_ref>       inspect one endpoint")
	fmt.Fprintln(w, "  capabilities                 list capability routing index")
}
