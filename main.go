// Command archbench is the cross-architecture benchmark & test orchestrator.
package main

import (
	"fmt"
	"os"

	"github.com/sirrobot01/archbench/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "archbench:", err)
		os.Exit(1)
	}
}
