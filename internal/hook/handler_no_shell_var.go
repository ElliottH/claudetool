package hook

import (
	"encoding/json"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func init() {
	Register("no-shell-var", handleNoShellVar)
}

// handleNoShellVar blocks a Bash command that assigns a literal shell variable
// and then expands it (e.g. `W=/long/path; grep ... $W/...`). Claude Code flags
// the $W as simple_expansion and prompts for approval, but a literal-valued
// variable saves nothing — inlining its value runs the command unprompted.
func handleNoShellVar(in *Input) (*Output, error) {
	if in.ToolName != "Bash" {
		return nil, nil
	}

	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(bash.Command), "")
	if err != nil {
		return nil, nil
	}

	type assign struct{ name, value string }
	var literals []assign
	expanded := map[string]bool{}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.Assign:
			// A naked assignment (`export X`) or index target has no inlinable value.
			if n.Name == nil || n.Value == nil {
				return true
			}
			// A value built from $(...)/backticks/other expansions genuinely
			// needs a variable, so only plain literals can be inlined.
			if val, ok := wordLiteral(n.Value); ok {
				literals = append(literals, assign{n.Name.Value, val})
			}
		case *syntax.ParamExp:
			expanded[n.Param.Value] = true
		}
		return true
	})

	for _, a := range literals {
		if expanded[a.name] {
			return nil, fmt.Errorf(
				"blocked: the command sets `%s=%s` and then expands `$%s`, which Claude Code flags as "+
					"simple_expansion and prompts for approval. The variable saves nothing here — "+
					"inline the literal value (%s) and drop the assignment.",
				a.name, a.value, a.name, a.value,
			)
		}
	}
	return nil, nil
}

// wordLiteral returns a word's value when it is composed purely of literal and
// quoted-literal parts (no parameter/command/arithmetic expansion), so the
// caller can treat it as a concrete string.
func wordLiteral(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			for _, inner := range p.Parts {
				lit, ok := inner.(*syntax.Lit)
				if !ok {
					return "", false
				}
				b.WriteString(lit.Value)
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}
