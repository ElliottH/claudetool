package hook

import (
	"encoding/json"
	"strings"
	"testing"
)

func makeBashInputCWD(command, cwd string) string {
	ti, _ := json.Marshal(BashInput{Command: command})
	in := Input{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     ti,
		CWD:           cwd,
	}
	data, _ := json.Marshal(in)
	return string(data)
}

func TestGitRedundantCBlocks(t *testing.T) {
	const cwd = "/Users/me/src/wearedev"
	tests := []struct {
		name    string
		command string
	}{
		{"absolute equal to cwd", "git -C /Users/me/src/wearedev status --short"},
		{"dot", "git -C . status"},
		{"dot slash", "git -C ./ log"},
		{"trailing slash", "git -C /Users/me/src/wearedev/ diff"},
		{"single quoted", "git -C '/Users/me/src/wearedev' status"},
		{"double quoted", `git -C "/Users/me/src/wearedev" status`},
		{"attached form", "git -C/Users/me/src/wearedev status"},
		{"redundant in a chain", "ls && git -C /Users/me/src/wearedev status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stderr := runHandler(t, "no-redundant-git-c", makeBashInputCWD(tt.command, cwd))
			if code != 2 {
				t.Errorf("exit code = %d, want 2 (block); stderr: %s", code, stderr)
			}
			if !strings.Contains(stderr, "redundant `git -C") {
				t.Errorf("stderr = %q, want redundant -C message", stderr)
			}
		})
	}
}

func TestGitRedundantCAllows(t *testing.T) {
	const cwd = "/Users/me/src/wearedev"
	tests := []struct {
		name    string
		command string
	}{
		{"no -C", "git status --short"},
		{"-C to a different absolute dir", "git -C /Users/me/src/other status"},
		{"-C to a subdirectory", "git -C /Users/me/src/wearedev/sub status"},
		{"-C to a relative subdirectory", "git -C sub status"},
		{"-C to parent", "git -C .. status"},
		{"not a git command", "grep -C 3 wearedev file.txt"},
		{"git -C inside a string literal", `echo "git -C /Users/me/src/wearedev status"`},
		{"git -C in a comment", "ls # git -C /Users/me/src/wearedev status"},
		{"dir is an unexpanded variable", "git -C $REPO status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stderr := runHandler(t, "no-redundant-git-c", makeBashInputCWD(tt.command, cwd))
			if code != 0 {
				t.Errorf("exit code = %d, want 0 (allow); stderr: %s", code, stderr)
			}
		})
	}
}

func TestGitRedundantCNoCWD(t *testing.T) {
	code, _ := runHandler(t, "no-redundant-git-c", makeBashInputCWD("git -C . status", ""))
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no cwd to compare)", code)
	}
}

func TestGitRedundantCIgnoresNonBash(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/tmp/x.txt",
		Content:  "git -C . status",
	})
	code, _ := runHandler(t, "no-redundant-git-c", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (ignore non-Bash)", code)
	}
}
