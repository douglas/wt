package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// copyFilesToWorktree copies files listed in paths from mainPath to wtPath.
// Missing source files are skipped with a warning. Write failures return an error.
// Paths that escape the source or destination directory are rejected.
// Symlinks are skipped to prevent following links to unintended files.
func copyFilesToWorktree(mainPath, wtPath string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	// Resolve symlinks in root paths so the bounds check cannot be bypassed
	// by a symlinked parent directory (e.g., repo -> /etc).
	resolvedMain, err := filepath.EvalSymlinks(mainPath)
	if err != nil {
		return fmt.Errorf("copy_files: resolve main path: %w", err)
	}
	resolvedWT, err := filepath.EvalSymlinks(wtPath)
	if err != nil {
		return fmt.Errorf("copy_files: resolve worktree path: %w", err)
	}

	for _, rel := range paths {
		src := filepath.Join(resolvedMain, rel)
		dst := filepath.Join(resolvedWT, rel)

		// Validate paths stay within their respective roots.
		// Use Abs to normalize after Join (handles .. in rel).
		absSrc, _ := filepath.Abs(src)
		absDst, _ := filepath.Abs(dst)
		if !isChildPath(absSrc, resolvedMain) {
			fmt.Fprintf(os.Stderr, "Warning: copy_files: %s escapes main worktree, skipping\n", rel)
			continue
		}
		if !isChildPath(absDst, resolvedWT) {
			fmt.Fprintf(os.Stderr, "Warning: copy_files: %s escapes worktree directory, skipping\n", rel)
			continue
		}

		// Use Lstat to detect symlinks without following them
		info, err := os.Lstat(src)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: copy_files: %s not found in main worktree, skipping\n", rel)
				continue
			}
			return fmt.Errorf("copy_files: stat %s: %w", rel, err)
		}

		// Skip symlinks to prevent following links to unintended files
		if info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(os.Stderr, "Warning: copy_files: %s is a symlink, skipping\n", rel)
			continue
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

// isChildPath checks if child is equal to or a subdirectory of parent.
func isChildPath(child, parent string) bool {
	return child == parent || strings.HasPrefix(child, parent+string(os.PathSeparator))
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
