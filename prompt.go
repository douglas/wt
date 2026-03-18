package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// openTTY opens /dev/tty for reading user input directly, bypassing
// stdin which may be captured by shell command substitution.
// Falls back to os.Stdin on Windows or if /dev/tty is unavailable.
func openTTY() (*os.File, error) {
	if os.Getenv("WT_USE_STDIN") == "1" {
		return os.Stdin, nil
	}
	f, err := os.Open("/dev/tty")
	if err != nil {
		return os.Stdin, nil
	}
	return f, nil
}

// selectItem displays a numbered list of items and reads the user's
// choice from /dev/tty. Returns the selected index and value.
func selectItem(label string, items []string) (int, string, error) {
	tty, err := openTTY()
	if err != nil {
		return -1, "", err
	}
	if tty != os.Stdin {
		defer tty.Close()
	}

	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, item)
	}
	fmt.Fprintf(os.Stderr, "Enter number [1-%d]: ", len(items))

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return -1, "", fmt.Errorf("selection cancelled")
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return -1, "", fmt.Errorf("selection cancelled")
	}

	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(items) {
		return -1, "", fmt.Errorf("invalid selection: %s", line)
	}

	return n - 1, items[n-1], nil
}

// confirmPrompt asks the user a yes/no question via /dev/tty.
// Returns true if the user answers "y" or "Y".
func confirmPrompt(label string) (bool, error) {
	tty, err := openTTY()
	if err != nil {
		return false, err
	}
	if tty != os.Stdin {
		defer tty.Close()
	}

	fmt.Fprintf(os.Stderr, "%s [y/N]: ", label)

	scanner := bufio.NewScanner(tty)
	if !scanner.Scan() {
		return false, nil
	}
	line := strings.TrimSpace(scanner.Text())

	return strings.EqualFold(line, "y"), nil
}
