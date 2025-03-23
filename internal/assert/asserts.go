package assert

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Equal compares two values
func Equal[T comparable](t *testing.T, want, got T) {
	t.Helper()

	// Log an error if the test does not pass the Equal test
	if want != got {
		t.Errorf("wanted: %v; got: %v", want, got)
	}
}

// NotEqual compares two values
func NotEqual[T comparable](t *testing.T, want, got T) {
	t.Helper()

	// Log an error if the test does not pass the Equal test
	if want == got {
		t.Errorf("wanted: %v; got: %v", want, got)
	}
}

// NoError asserts that the given error is nil
func NoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		message := "Expected no error, but got an error"
		if len(msgAndArgs) > 0 {
			message = fmt.Sprintf("%s: %s", message, fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...))
		}
		t.Errorf("%s\nError: %v", message, err)
	}
}

// EqualSlices compares two slices of any comparable type for equality
func EqualSlices[T comparable](t *testing.T, want, got []T) {
	t.Helper()
	// Compare the length of the slices for equality
	if len(want) != len(got) {
		t.Errorf("slice length mismatch: wanted %d elements; got %d elements", len(want), len(got))
		t.Errorf("wanted: %v; got: %v", want, got)
		return
	}

	// Compate each element
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("mismatch at index %d: wanted %v; got %v", i, want[i], got[i])
		}
	}
}

// StringIn tests if a string contains a specified substring
func StringIn(t *testing.T, want, inString string) {
	t.Helper()

	if !strings.Contains(inString, want) {
		t.Errorf("wanted %q; in: %q", want, inString)
	}
}

// StringNotIn tests that the string does not contain a specified substring
func StringNotIn(t *testing.T, dontWant, inString string) {
	t.Helper()

	if strings.Contains(inString, dontWant) {
		t.Errorf("dont want %q; in: %q", dontWant, inString)
	}
}

// EqualTime tests if the time is equal (times are within allowedDiff of each other)
func EqualTime(t *testing.T, want, got time.Time, allowedDiff time.Duration) {
	t.Helper()
	dif := got.Sub(want).Abs()

	if dif > allowedDiff {
		t.Errorf("wanted %v; not %v; off by %v seconds", want, got, dif.Seconds())
	}
}
