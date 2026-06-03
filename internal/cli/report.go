package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/sirrobot01/archbench/internal/engine"
	"github.com/spf13/cobra"
)

func newReportCmd() *cobra.Command {
	var (
		dir    string
		format string
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render stored result artifacts (term|md)",
		RunE: func(cmd *cobra.Command, args []string) error {
			files, err := filepath.Glob(filepath.Join(dir, "*.json"))
			if err != nil {
				return err
			}
			if len(files) == 0 {
				return fmt.Errorf("no result artifacts in %s (run `archbench run` first)", dir)
			}
			sort.Strings(files)

			for _, f := range files {
				r, err := readResult(f)
				if err != nil {
					return err
				}
				switch format {
				case "term":
					engine.Terminal(cmd, r)
				case "md":
					engine.Markdown(cmd, r)
				default:
					return fmt.Errorf("unknown format %q (want term|md)", format)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "archbench-results", "directory of result artifacts")
	cmd.Flags().StringVarP(&format, "format", "f", "term", "output format: term|md")
	return cmd
}
