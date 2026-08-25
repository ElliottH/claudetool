package hook

import (
	"strings"
	"testing"
)

func TestNoShellVarBlocks(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"assign then expand", "W=/Users/me/src/wearedev; grep -rn foo $W/proto/*.go"},
		{"braced expansion", "W=/tmp; ls ${W}/x"},
		{"expanded twice", "D=/etc; cat $D/hosts; ls $D"},
		{"separated by newline", "W=/tmp\nls $W/x"},
		{"expanded inside double quotes", `W=/tmp; ls "$W/x"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-shell-var", input)
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr: %s", code, stderr)
			}
			if !strings.Contains(stderr, "simple_expansion") {
				t.Errorf("stderr = %q, want simple_expansion message", stderr)
			}
		})
	}
}

func TestNoShellVarAllows(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{"inlined literal", "grep -rn foo /Users/me/src/wearedev/proto/*.go"},
		{"command substitution", "RESULT=$(git rev-parse HEAD); echo $RESULT"},
		{"value from another var", "X=$OTHER; echo $X"},
		{"env prefix, no expansion", "FOO=bar make build"},
		{"for loop variable", "for f in *.go; do echo $f; done"},
		{"assigned but never expanded", "W=/tmp; ls /etc"},
		{"expands a different var", "W=/tmp; echo $WORD"},
		{"var name only appears inside single quotes", "W=/tmp; echo '$W'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: tt.command})
			code, stderr := runHandler(t, "no-shell-var", input)
			if code != 0 {
				t.Errorf("exit code = %d, want 0 (allow); stderr: %s", code, stderr)
			}
		})
	}
}

func TestNoShellVarIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/tmp/x.sh",
		Content:  "W=/tmp; ls $W",
	})
	code, _ := runHandler(t, "no-shell-var", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (ignore non-Bash)", code)
	}
}
