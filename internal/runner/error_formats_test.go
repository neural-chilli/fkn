package runner

import "testing"

func TestParseEslintErrorsIgnoresNonErrorNoise(t *testing.T) {
	t.Parallel()

	stderr := `
/tmp/src/app.ts
  3:5  warning  unused variable  no-unused-vars
  4:2  error  missing semicolon  semi
✖ 1 problem (1 error, 0 warnings)
`

	errors := parseEslintErrors(stderr)
	if len(errors) != 1 {
		t.Fatalf("len(errors) = %d, want 1", len(errors))
	}
	if errors[0].Message != "4:2  error  missing semicolon  semi" {
		t.Fatalf("errors[0] = %+v, want eslint error summary", errors[0])
	}
}
