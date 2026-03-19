package main

import (
	"os"
	"testing"
)

func TestOpenTTY_UseStdin(t *testing.T) {
	t.Setenv("WT_USE_STDIN", "1")
	f := openTTY()
	if f.Fd() != os.Stdin.Fd() {
		t.Errorf("expected fd %d (os.Stdin), got fd %d", os.Stdin.Fd(), f.Fd())
	}
}

func TestOpenTTY_DevTTY(t *testing.T) {
	t.Setenv("WT_USE_STDIN", "")

	// /dev/tty is unavailable in CI and headless environments.
	f, err := os.Open("/dev/tty")
	if err != nil {
		t.Skip("/dev/tty unavailable, skipping")
	}
	f.Close()

	got := openTTY()
	defer got.Close()
	if got.Fd() == os.Stdin.Fd() {
		t.Error("expected openTTY to return /dev/tty, got os.Stdin")
	}
}

func pipeStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	if input != "" {
		_, err = w.WriteString(input)
		if err != nil {
			t.Fatalf("WriteString failed: %v", err)
		}
	}
	w.Close()
	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = origStdin })
}

func TestSelectItem(t *testing.T) {
	items := []string{"alpha", "beta", "gamma"}

	tests := []struct {
		name    string
		input   string
		items   []string
		wantIdx int
		wantVal string
		wantErr string
	}{
		{
			name:    "valid selection 2",
			input:   "2\n",
			items:   items,
			wantIdx: 1,
			wantVal: "beta",
		},
		{
			name:    "valid selection 1",
			input:   "1\n",
			items:   items,
			wantIdx: 0,
			wantVal: "alpha",
		},
		{
			name:    "out of bounds",
			input:   "5\n",
			items:   items,
			wantIdx: -1,
			wantErr: "invalid selection",
		},
		{
			name:    "non-numeric input",
			input:   "abc\n",
			items:   items,
			wantIdx: -1,
			wantErr: "invalid selection",
		},
		{
			name:    "empty input",
			input:   "\n",
			items:   items,
			wantIdx: -1,
			wantErr: "selection cancelled",
		},
		{
			name:    "EOF no input",
			input:   "",
			items:   items,
			wantIdx: -1,
			wantErr: "selection cancelled",
		},
		{
			name:    "empty items list",
			input:   "1\n",
			items:   []string{},
			wantIdx: -1,
			wantErr: "invalid selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WT_USE_STDIN", "1")
			pipeStdin(t, tt.input)

			idx, val, err := selectItem("Pick one", tt.items)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if got := err.Error(); !contains(got, tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, got)
				}
				if idx != tt.wantIdx {
					t.Errorf("expected index %d, got %d", tt.wantIdx, idx)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if idx != tt.wantIdx {
				t.Errorf("expected index %d, got %d", tt.wantIdx, idx)
			}
			if val != tt.wantVal {
				t.Errorf("expected value %q, got %q", tt.wantVal, val)
			}
		})
	}
}

func TestConfirmPrompt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase Y", input: "Y\n", want: true},
		{name: "lowercase n", input: "n\n", want: false},
		{name: "no", input: "no\n", want: false},
		{name: "empty input", input: "\n", want: false},
		{name: "EOF", input: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("WT_USE_STDIN", "1")
			pipeStdin(t, tt.input)

			got, err := confirmPrompt("Continue?")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
