// Package cli defines the archbench command tree.
package cli

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench/internal/parser/gotest"
	"github.com/sirrobot01/archbench/spec"
)

// version is overwritten at release time via -ldflags -X. For `go install`
// builds it stays "dev" here, so buildVersion falls back to the module version
// the Go toolchain embeds in the binary.
var version = "dev"

// buildVersion reports the release version when set via ldflags, otherwise the
// module version recorded by `go install` (e.g. a tag or pseudo-version).
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// Execute runs the root command. The context is cancelled on SIGINT/SIGTERM so
// in-flight runs can stop and clean up.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := &cobra.Command{
		Use:           "archbench",
		Short:         "Cross-architecture benchmark and test orchestration",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newInitCmd(), newRunCmd(), newGenerateCmd(), newCompareCmd(), newReportCmd(), newCacheCmd(), newExecCmd(), newVersionCmd())
	return root.ExecuteContext(ctx)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the archbench version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(buildVersion())
		},
	}
}

// registry returns the built-in parsers.
func registry() *spec.Registry {
	reg := spec.NewRegistry()
	reg.Register(gotest.New())
	return reg
}
