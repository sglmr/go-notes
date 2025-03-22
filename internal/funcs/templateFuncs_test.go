package funcs

import (
	"testing"

	"github.com/sglmr/go-notes/internal/assert"
)

// TestSlugify runs a series of tests on the slugify function
func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Hello_World", "hello_world"},
		{"Hello-World", "hello-world"},
		{"Hello World 123", "hello-world-123"},
		{"Hello   World", "hello---world"},
		{"HELLO world", "hello-world"},
		{"Hello, World!", "hello-world"},
		{"Héllö Wörld", "hll-wrld"},
		{"", ""},
		{"---", "---"},
		{"Special@#$Characters", "specialcharacters"},
		{"Mixed 123 & Symbols!", "mixed-123--symbols"},
		{"  Leading and trailing spaces  ", "--leading-and-trailing-spaces--"},
		{"123 Start with number", "123-start-with-number"},
		{"CamelCaseExample", "camelcaseexample"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, slugify(test.input))
		})
	}
}
