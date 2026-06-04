package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench/spec"
)

const exampleSpec = `name: my-suite

mode: bench            # bench | test

targets:
  - name: local
    type: local

  # - name: amd64-box
  #   type: ssh
  #   host: 10.0.0.5
  #   user: ubuntu

  # - name: amd64-container
  #   type: docker
  #   image: golang:1.26
  #   platform: linux/amd64   # optional; pins a non-native arch via emulation

  # - name: ci
  #   type: github-actions
  #   runsOn: ubuntu-latest   # used by 'archbench generate'

runs:
  - name: all
    setup:
      - go mod download
    command: go test ./... -run '^$' -bench=. -benchmem -count=10

parser: go-test
`

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate an example archbench.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := spec.DefaultSpecFile
			if _, err := os.Stat(path); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", path)
			}
			if err := os.WriteFile(path, []byte(exampleSpec), 0o644); err != nil {
				return err
			}
			cmd.Printf("wrote %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing spec")
	return cmd
}
