package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// errCancelled is a package-level alias for ErrCancelled (defined in errors.go).
var errCancelled = ErrCancelled

// openTTY opens /dev/tty for reading user input directly, bypassing
// stdin which may be captured by shell command substitution.
// Falls back to os.Stdin on Windows or if /dev/tty is unavailable.
func openTTY() *os.File {
	if os.Getenv("WT_USE_STDIN") == "1" {
		return os.Stdin
	}
	f, err := os.Open("/dev/tty")
	if err != nil {
		return os.Stdin
	}
	return f
}

// readLineRaw reads a line of input in raw mode, echoing typed digits to
// stderr. Supports backspace, Enter, ESC, and Ctrl-C. Falls back to
// bufio.Scanner if the terminal cannot be put into raw mode (e.g. piped input).
func readLineRaw(tty *os.File) (string, error) {
	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: non-TTY (piped input in tests).
		scanner := bufio.NewScanner(tty)
		if !scanner.Scan() {
			return "", errCancelled
		}
		line := strings.TrimSpace(scanner.Text())
		// Detect ESC or Ctrl-C bytes in buffered input.
		if strings.ContainsAny(line, "\x1b\x03") {
			return "", errCancelled
		}
		return line, nil
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	var buf []byte
	b := make([]byte, 1)
	for {
		_, err := tty.Read(b)
		if err != nil {
			return "", errCancelled
		}

		switch {
		case b[0] == 0x1b: // ESC
			return "", errCancelled
		case b[0] == 0x03: // Ctrl-C
			return "", errCancelled
		case b[0] == '\r' || b[0] == '\n': // Enter
			fmt.Fprint(os.Stderr, "\r\n")
			return string(buf), nil
		case b[0] == 0x7f: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				// Move cursor back, overwrite with space, move back again.
				fmt.Fprint(os.Stderr, "\b \b")
			}
		case b[0] >= '0' && b[0] <= '9': // Digit
			buf = append(buf, b[0])
			fmt.Fprint(os.Stderr, string(b[0]))
		}
		// Ignore all other bytes.
	}
}

// readConfirmRaw reads a single confirmation character in raw mode.
// Returns the character typed, or 0 on ESC/Ctrl-C/EOF. Falls back to
// bufio.Scanner if the terminal cannot be put into raw mode.
func readConfirmRaw(tty *os.File) (byte, error) {
	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: non-TTY (piped input in tests).
		scanner := bufio.NewScanner(tty)
		if !scanner.Scan() {
			return 0, nil
		}
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			return '\n', nil
		}
		// Detect ESC or Ctrl-C bytes in buffered input.
		if line[0] == 0x1b || line[0] == 0x03 {
			return 0, nil
		}
		return line[0], nil
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	b := make([]byte, 1)
	_, err = tty.Read(b)
	if err != nil {
		return 0, nil
	}

	switch {
	case b[0] == 0x1b: // ESC
		return 0, nil
	case b[0] == 0x03: // Ctrl-C
		return 0, nil
	default:
		fmt.Fprint(os.Stderr, string(b[0])+"\r\n")
		return b[0], nil
	}
}

// selectItem displays a numbered list of items and reads the user's
// choice from /dev/tty. Returns the selected index and value.
func selectItem(label string, items []string) (int, string, error) {
	tty := openTTY()
	if tty != os.Stdin {
		defer tty.Close()
	}

	fmt.Fprintf(os.Stderr, "%s:\n", label)
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, item)
	}
	fmt.Fprintf(os.Stderr, "Enter number [1-%d]: ", len(items))

	line, err := readLineRaw(tty)
	if err != nil {
		return -1, "", err
	}
	if line == "" {
		return -1, "", errCancelled
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
	tty := openTTY()
	if tty != os.Stdin {
		defer tty.Close()
	}

	fmt.Fprintf(os.Stderr, "%s [y/N]: ", label)

	ch, err := readConfirmRaw(tty)
	if err != nil {
		return false, err
	}

	return ch == 'y' || ch == 'Y', nil
}
