package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func init() {
	Register("no-redundant-git-c", handleNoRedundantGitC)
}

// handleNoRedundantGitC blocks `git -C <dir>` when the shell is already in <dir>:
// the -C needs approval as potentially unsafe, but pointing it at the current
// directory is a no-op, so dropping it lets the command run unprompted. Unlike
// no-git-c, it leaves `git -C` against a different directory alone.
func handleNoRedundantGitC(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	if in.CWD == "" {
		return nil, nil
	}
	cwd := filepath.Clean(in.CWD)

	file, err := syntax.NewParser().Parse(strings.NewReader(bash.Command), "")
	if err != nil {
		// A command we can't parse is one we can't reason about safely, so
		// leave it for Claude Code's own approval flow rather than block it.
		return nil, nil
	}

	var blockErr error
	syntax.Walk(file, func(node syntax.Node) bool {
		if blockErr != nil {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		raw, ok := gitDashCDir(call)
		if !ok {
			return true
		}
		dir := expandHome(raw)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		if filepath.Clean(dir) == cwd {
			blockErr = fmt.Errorf(
				"blocked: redundant `git -C %s` — the shell is already in that directory, "+
					"so the -C is a no-op that only forces an approval prompt. Drop `-C %s` and run git directly.",
				raw, raw,
			)
		}
		return true
	})
	return nil, blockErr
}

// gitDashCDir reports the directory passed to `git -C <dir>` in a single
// command, handling both the separated (`-C dir`) and attached (`-C/dir`)
// forms. It returns ok=false when the call isn't git, has no -C, or the
// directory isn't a plain literal (an expansion like `$HOME` can't be compared
// against the cwd at hook time).
func gitDashCDir(call *syntax.CallExpr) (dir string, ok bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	if name, isLit := wordLiteral(call.Args[0]); !isLit || name != "git" {
		return "", false
	}
	for i := 1; i < len(call.Args); i++ {
		arg, isLit := wordLiteral(call.Args[i])
		if !isLit {
			continue
		}
		switch {
		case arg == "-C":
			if i+1 < len(call.Args) {
				return wordLiteral(call.Args[i+1])
			}
		case strings.HasPrefix(arg, "-C") && len(arg) > 2:
			return arg[2:], true
		}
	}
	return "", false
}

// expandHome expands a leading ~ or ~/ to the user's home directory, matching
// what the shell would do before git sees the path.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
