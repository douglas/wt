package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyFilesToWorktree copies files listed in paths from mainPath to wtPath.
// Missing source files are skipped with a warning. Write failures return an error.
func copyFilesToWorktree(mainPath, wtPath string, paths []string) error {
	for _, rel := range paths {
		src := filepath.Join(mainPath, rel)
		dst := filepath.Join(wtPath, rel)

		info, err := os.Stat(src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: copy_files: %s not found in main worktree, skipping\n", rel)
				continue
			}
			return fmt.Errorf("copy_files: stat %s: %w", rel, err)
		}

		// Create parent directories if needed
		if dir := filepath.Dir(dst); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("copy_files: mkdir %s: %w", dir, err)
			}
		}

		if err := copyFile(src, dst, info.Mode()); err != nil {
			return fmt.Errorf("copy_files: %s: %w", rel, err)
		}
	}
	return nil
}

// copyFile copies a single file from src to dst, preserving the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
