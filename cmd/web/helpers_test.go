package main

import (
	"reflect"
	"testing"
)

func TestExtractTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "No hashtags",
			input:    "This is a text without hashtags",
			expected: []string{},
		},
		{
			name:     "Single valid hashtag",
			input:    "This is a #hashtag",
			expected: []string{"hashtag"},
		},
		{
			name:     "Multiple valid hashtags",
			input:    "This is #one and this is #two",
			expected: []string{"one", "two"},
		},
		{
			name:     "Numbers only hashtag (#2) - should not match",
			input:    "This is #2",
			expected: []string{},
		},
		{
			name:     "Numbers only hashtag (#123) - should not match",
			input:    "This is #123",
			expected: []string{},
		},
		{
			name:     "Mixed hashtag with at least one a-z (#12a123) - should match",
			input:    "This is #12a123",
			expected: []string{"12a123"},
		},
		{
			name:     "Hashtag at beginning of text",
			input:    "#beginning of text",
			expected: []string{"beginning"},
		},
		{
			name:     "Hashtags with hyphens",
			input:    "This is #with-hyphen",
			expected: []string{"with-hyphen"},
		},
		{
			name:     "Hashtag with uppercase letters - should not match",
			input:    "This is #HashTag",
			expected: []string{},
		},
		{
			name:     "Markdown link - should be excluded",
			input:    "This is a [link](#heading-link)",
			expected: []string{},
		},
		{
			name:     "HTML link - should be excluded",
			input:    "This is a <a href=\"#heading-link\">link</a>",
			expected: []string{},
		},
		{
			name:     "Single char hashtag - should be excluded",
			input:    "This is #a",
			expected: []string{},
		},
		{
			name:     "Various valid and invalid hashtags mixed",
			input:    "Valid: #valid #valid-one #v123 Invalid: #123 #A #",
			expected: []string{"valid", "valid-one", "v123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTags(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("extractTags(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
