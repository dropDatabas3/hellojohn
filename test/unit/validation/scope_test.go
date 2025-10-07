package validation

import (
	"testing"

	"github.com/dropDatabas3/hellojohn/internal/validation"
)

func TestValidScopeName_Valid(t *testing.T) {
	valids := []string{
		"a",
		"ab",
		"profile",
		"profile:read",
		"email:read:e2e123",
		"a_b-c.d:scope2",
		// 64 chars (start/end alnum)
		mkLen("a", 62) + "b", // a + 62 x 'a' + b
	}
	for _, v := range valids {
		if !validation.ValidScopeName(v) {
			t.Fatalf("expected valid: %q", v)
		}
	}
}

func TestValidScopeName_Invalid(t *testing.T) {
	invalids := []string{
		"",               // empty
		":lead",          // starts with non-alnum
		"trail:",         // ends with non-alnum
		"bad space",      // space
		"UPPER",          // uppercase
		"semicolon;hack", // semicolon
		mkLen("a", 65),   // > 64 chars should be invalid
		mkLen("a", 100),  // way too long
	}
	for _, v := range invalids {
		if validation.ValidScopeName(v) {
			t.Fatalf("expected invalid: %q", v)
		}
	}
}

// mkLen builds a string of exactly n 'a' characters, optionally with given prefix if provided in future.
func mkLen(prefix string, total int) string {
	if total <= len(prefix) {
		return prefix[:total]
	}
	out := make([]byte, total)
	copy(out, []byte(prefix))
	for i := len(prefix); i < total; i++ {
		out[i] = 'a'
	}
	return string(out)
}
