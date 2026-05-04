package tcctl

import (
	"context"
	"io"

	"github.com/nangman-infra/touch-connect/tcctl/internal/cli"
)

type ExitError = cli.ExitError

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	return cli.Run(ctx, args, stdout, stderr)
}
