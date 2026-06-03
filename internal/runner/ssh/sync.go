package ssh

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// excludedDirs are skipped when packaging the project: VCS metadata and
// result artifacts that would otherwise be re-uploaded.
var excludedDirs = map[string]bool{
	".git":              true,
	".hg":               true,
	".svn":              true,
	"archbench-results": true,
	"node_modules":      true,
}

// streamTar writes a gzip-compressed tar of root into w, then closes w.
func streamTar(w io.WriteCloser, root string) (err error) {
	defer func() {
		if cerr := w.Close(); err == nil {
			err = cerr
		}
	}()

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	entries, walkErr := archiveEntries(root)
	if walkErr == nil {
		for _, entry := range entries {
			if err := writeTarEntry(tw, root, entry); err != nil {
				walkErr = err
				break
			}
		}
	}

	if cerr := tw.Close(); walkErr == nil {
		walkErr = cerr
	}
	if cerr := gz.Close(); walkErr == nil {
		walkErr = cerr
	}
	return walkErr
}

func archiveEntries(root string) ([]string, error) {
	if entries, err := gitEntries(root); err == nil {
		return entries, nil
	}
	return walkEntries(root)
}

func gitEntries(root string) ([]string, error) {
	cmd := exec.Command("git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	raw := strings.Split(stdout.String(), "\x00")
	entries := make([]string, 0, len(raw))
	for _, rel := range raw {
		if rel == "" {
			continue
		}
		if !filepath.IsLocal(rel) {
			return nil, errUnsafePath(rel)
		}
		entries = append(entries, filepath.ToSlash(rel))
	}
	return entries, nil
}

func walkEntries(root string) ([]string, error) {
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if d.IsDir() && excludedDirs[d.Name()] {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !filepath.IsLocal(rel) {
			return errUnsafePath(rel)
		}
		entries = append(entries, filepath.ToSlash(rel))
		return nil
	})
	return entries, err
}

func writeTarEntry(tw *tar.Writer, root, rel string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	link := ""
	if info.Mode()&fs.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(rel)

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(tw, f)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func errUnsafePath(path string) error {
	return &fs.PathError{Op: "archive", Path: path, Err: fs.ErrInvalid}
}
