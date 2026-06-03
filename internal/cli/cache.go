package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sirrobot01/archbench"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local build cache",
	}
	cmd.AddCommand(newCacheDirCmd(), newCacheCleanCmd())
	return cmd
}

func newCacheDirCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dir",
		Short: "Print the local cache directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := localCacheRoot()
			if err != nil {
				return err
			}
			cmd.Println(dir)
			return nil
		},
	}
}

func newCacheCleanCmd() *cobra.Command {
	var suite string
	c := &cobra.Command{
		Use:   "clean",
		Short: "Remove the local cache, or a single suite with --suite",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := localCacheRoot()
			if err != nil {
				return err
			}
			if suite != "" {
				dir = filepath.Join(dir, archbench.Slug(suite))
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			cmd.Printf("removed %s\n", dir)
			return nil
		},
	}
	c.Flags().StringVar(&suite, "suite", "", "only clean this suite")
	return c
}

// localCacheRoot is where the local runner keeps persistent caches. Remote
// caches live on their hosts and are not managed here.
func localCacheRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "archbench"), nil
}
