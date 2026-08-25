package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGoSwallowedErrorBlocks(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	// Write a Go file with a swallowed error
	f, err := os.CreateTemp(t.TempDir(), "bad-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import "log/slog"

func doStuff() {
	err := something()
	if err != nil {
		slog.Warn("something failed")
	}
}

func something() error { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	var stdout, stderr bytes.Buffer
	code := Run([]string{"go-swallowed-error"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}

	var out Output
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout.String())
	}
	if out.Decision != "block" {
		t.Errorf("decision = %q, want block", out.Decision)
	}
	if !strings.Contains(out.Reason, "swallowed") {
		t.Errorf("reason = %q, want mention of swallowed error", out.Reason)
	}
}

func TestGoSwallowedErrorAllowsCommented(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	// A comment above the log call justifies the swallowed error
	f, err := os.CreateTemp(t.TempDir(), "commented-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import "log/slog"

func doStuff() {
	err := something()
	if err != nil {
		// best-effort: failure here doesn't affect the caller
		slog.Warn("something failed")
	}
}

func something() error { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (commented swallow is OK); stderr: %s", code, stderr)
	}
}

func TestGoSwallowedErrorAllowsAugmentedSlogParams(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	f, err := os.CreateTemp(t.TempDir(), "augment-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import (
	"context"
	"log/slog"
)

func doStuff(ctx context.Context) error {
	err := something()
	if err != nil {
		return Augment(err, "something failed", slog.Params(ctx))
	}
	return nil
}

func Augment(err error, msg string, args ...any) error { return err }
func something() error                                  { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (error is propagated, not swallowed); stderr: %s", code, stderr)
	}
}

func TestGoSwallowedErrorAllowsMultiValueAugmentedSlogParams(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	f, err := os.CreateTemp(t.TempDir(), "multivalue-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import (
	"context"
	"log/slog"
)

func doStuff(ctx context.Context) (int, error) {
	err := something()
	if err != nil {
		return 0, Augment(err, "something failed", slog.Params(ctx))
	}
	return 1, nil
}

func Augment(err error, msg string, args ...any) error { return err }
func something() error                                  { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (error is propagated, not swallowed); stderr: %s", code, stderr)
	}
}

func TestGoSwallowedErrorAllowsWrappedPropagation(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	f, err := os.CreateTemp(t.TempDir(), "wrapped-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import (
	"fmt"
	"log/slog"
)

func doStuff() error {
	err := something()
	if err != nil {
		slog.Warn("something failed")
		return fmt.Errorf("doStuff: %w", err)
	}
	return nil
}

func something() error { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (error is wrapped and returned); stderr: %s", code, stderr)
	}
}

func TestGoSwallowedErrorAllowsStructWrappedPropagation(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	f, err := os.CreateTemp(t.TempDir(), "structwrapped-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

import "fmt"

type Response struct{ Error error }

func doStuff() Response {
	if err := something(); err != nil {
		return Response{Error: Augment(err, "posting chunk report", map[string]string{
			"chunk_index": fmt.Sprint(1),
		})}
	}
	return Response{}
}

func Augment(err error, msg string, args ...any) error { return err }
func something() error                                 { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (error is propagated via struct literal); stderr: %s", code, stderr)
	}
}

func TestGoSwallowedErrorAllowsPropagated(t *testing.T) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		t.Skip("semgrep not installed")
	}

	// Write a Go file that properly returns the error
	f, err := os.CreateTemp(t.TempDir(), "good-*.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`package main

func doStuff() error {
	err := something()
	if err != nil {
		return err
	}
	return nil
}

func something() error { return nil }
`)
	f.Close()

	input := makeToolInput("PostToolUse", "Write", WriteInput{
		FilePath: f.Name(),
		Content:  "ignored",
	})

	code, stderr := runHandler(t, "go-swallowed-error", input)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (clean file); stderr: %s", code, stderr)
	}
}
