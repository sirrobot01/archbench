// Package cli defines the archbench command tree.
package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
	"github.com/sirrobot01/archbench/internal/parser/gotest"
)

var version = "dev"

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
			cmd.Println(version)
		},
	}
}

// registry returns the built-in parsers.
func registry() *archbench.Registry {
	reg := archbench.NewRegistry()
	reg.Register(gotest.New())
	return reg
}
