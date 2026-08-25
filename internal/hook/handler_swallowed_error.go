package hook

func init() {
	Register("go-swallowed-error", semgrepHandler(goSwallowedErrorRule))
}

const goSwallowedErrorRule = `rules:
  - id: go-swallowed-error
    patterns:
      - pattern: |
          if err != nil {
              ...
              $LOG.$METHOD(...)
              ...
          }
      - metavariable-regex:
          metavariable: $LOG
          regex: ^(slog|log|fmt)$
      # Not swallowed if err is propagated in the return, however it is nested:
      # directly (return err / return nil, err), wrapped (return fmt.Errorf(..., err),
      # return terrors.Augment(err, ...)), or embedded in a struct literal
      # (return typhon.Response{Error: terrors.Augment(err, ...)}). The deep
      # expression operator matches err anywhere in the returned expression.
      # Without this, a log-package call nested as an argument to the returned
      # wrapper matches $LOG.$METHOD and produces a false positive.
      - pattern-not: |
          if err != nil {
              ...
              return <... err ...>
          }
      - pattern-not-regex: "//.*\n"
    languages: [go]
    severity: WARNING
    message: >-
      Error is logged but not propagated. Either return the error
      or add a comment explaining why it is safe to swallow.
`
