package main

import (
	"os"
	"strings"
	"testing"
)

// TestRewriteDeprecatedCommand_AllAliases verifies that each deprecated command
// rewrites os.Args to the expected new command with implicit flags prepended.
func TestRewriteDeprecatedCommand_AllAliases(t *testing.T) {
	tests := []struct {
		oldCommand     string
		inputArgs      []string
		wantCommand    string
		wantArgs       []string
		wantNoticeWord string // substring expected in the deprecation notice
	}{
		{
			oldCommand:     "sync",
			inputArgs:      []string{"git-vendor", "sync", "--force"},
			wantCommand:    "pull",
			wantArgs:       []string{"git-vendor", "pull", "--locked", "--force"},
			wantNoticeWord: "pull --locked",
		},
		{
			oldCommand:     "update",
			inputArgs:      []string{"git-vendor", "update", "myvendor"},
			wantCommand:    "pull",
			wantArgs:       []string{"git-vendor", "pull", "myvendor"},
			wantNoticeWord: "'git vendor update' is now 'git vendor pull'",
		},
		{
			oldCommand:     "verify",
			inputArgs:      []string{"git-vendor", "verify", "--json"},
			wantCommand:    "status",
			wantArgs:       []string{"git-vendor", "status", "--offline", "--json"},
			wantNoticeWord: "status --offline",
		},
		{
			oldCommand:     "diff",
			inputArgs:      []string{"git-vendor", "diff", "myvendor", "--ref", "main"},
			wantCommand:    "status",
			wantArgs:       []string{"git-vendor", "status", "myvendor", "--ref", "main"},
			wantNoticeWord: "'git vendor diff' is now 'git vendor status'",
		},
		{
			oldCommand:     "outdated",
			inputArgs:      []string{"git-vendor", "outdated", "--json"},
			wantCommand:    "status",
			wantArgs:       []string{"git-vendor", "status", "--remote-only", "--json"},
			wantNoticeWord: "status --remote-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.oldCommand, func(t *testing.T) {
			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Set os.Args and run rewrite
			os.Args = make([]string, len(tt.inputArgs))
			copy(os.Args, tt.inputArgs)

			got := rewriteDeprecatedCommand(tt.oldCommand)

			// Restore stderr and read captured output
			w.Close()
			os.Stderr = oldStderr
			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			r.Close()
			stderr := string(buf[:n])

			// Verify command was rewritten
			if got != tt.wantCommand {
				t.Errorf("rewriteDeprecatedCommand(%q) returned %q, want %q", tt.oldCommand, got, tt.wantCommand)
			}

			// Verify os.Args was rewritten
			if len(os.Args) != len(tt.wantArgs) {
				t.Errorf("os.Args length = %d, want %d\n  got:  %v\n  want: %v", len(os.Args), len(tt.wantArgs), os.Args, tt.wantArgs)
			} else {
				for i := range tt.wantArgs {
					if os.Args[i] != tt.wantArgs[i] {
						t.Errorf("os.Args[%d] = %q, want %q", i, os.Args[i], tt.wantArgs[i])
					}
				}
			}

			// Verify deprecation notice was printed to stderr
			if !strings.Contains(stderr, "DEPRECATED") {
				t.Errorf("expected DEPRECATED in stderr, got: %q", stderr)
			}
			if !strings.Contains(stderr, tt.wantNoticeWord) {
				t.Errorf("expected %q in stderr, got: %q", tt.wantNoticeWord, stderr)
			}
		})
	}
}

// TestRewriteDeprecatedCommand_NonDeprecated verifies that non-deprecated
// commands pass through unchanged with no stderr output.
func TestRewriteDeprecatedCommand_NonDeprecated(t *testing.T) {
	nonDeprecated := []string{"init", "add", "pull", "status", "list", "remove", "watch", "config"}

	for _, cmd := range nonDeprecated {
		t.Run(cmd, func(t *testing.T) {
			originalArgs := []string{"git-vendor", cmd, "--json"}
			os.Args = make([]string, len(originalArgs))
			copy(os.Args, originalArgs)

			got := rewriteDeprecatedCommand(cmd)

			if got != cmd {
				t.Errorf("rewriteDeprecatedCommand(%q) = %q, want unchanged", cmd, got)
			}

			// os.Args should be unchanged
			if len(os.Args) != len(originalArgs) {
				t.Errorf("os.Args was modified for non-deprecated command %q", cmd)
			}
		})
	}
}

// TestRewriteDeprecatedCommand_NoTrailingArgs verifies that aliases work
// correctly when no additional arguments are provided (bare command).
func TestRewriteDeprecatedCommand_NoTrailingArgs(t *testing.T) {
	tests := []struct {
		oldCommand string
		wantArgs   []string
	}{
		{"sync", []string{"git-vendor", "pull", "--locked"}},
		{"update", []string{"git-vendor", "pull"}},
		{"verify", []string{"git-vendor", "status", "--offline"}},
		{"diff", []string{"git-vendor", "status"}},
		{"outdated", []string{"git-vendor", "status", "--remote-only"}},
	}

	for _, tt := range tests {
		t.Run(tt.oldCommand+"_bare", func(t *testing.T) {
			oldStderr := os.Stderr
			_, w, _ := os.Pipe()
			os.Stderr = w

			os.Args = []string{"git-vendor", tt.oldCommand}
			rewriteDeprecatedCommand(tt.oldCommand)

			w.Close()
			os.Stderr = oldStderr

			if len(os.Args) != len(tt.wantArgs) {
				t.Errorf("os.Args = %v, want %v", os.Args, tt.wantArgs)
			} else {
				for i := range tt.wantArgs {
					if os.Args[i] != tt.wantArgs[i] {
						t.Errorf("os.Args[%d] = %q, want %q", i, os.Args[i], tt.wantArgs[i])
					}
				}
			}
		})
	}
}

// TestDeprecatedCommandsMapCompleteness ensures every entry in the
// deprecatedCommands map has a matching entry in the cmd.DeprecatedCommands
// map used by shell completion, and vice versa.
func TestDeprecatedCommandsMapCompleteness(t *testing.T) {
	for cmd := range deprecatedCommands {
		if _, ok := deprecatedCommands[cmd]; !ok {
			t.Errorf("deprecatedCommands has %q but deprecatedCommands map is missing it", cmd)
		}
	}

	// Verify all 5 expected aliases are present
	expected := []string{"sync", "update", "verify", "diff", "outdated"}
	for _, cmd := range expected {
		if _, ok := deprecatedCommands[cmd]; !ok {
			t.Errorf("expected deprecated alias for %q, not found in deprecatedCommands", cmd)
		}
	}
}
