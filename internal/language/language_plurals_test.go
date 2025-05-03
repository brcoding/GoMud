package language

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluralize(t *testing.T) {
	tests := []struct {
		word     string
		language string
		expected string
	}{
		{"knife", "en", "knives"},
		{"wolf", "en", "wolves"},
		{"analysis", "en", "analyses"},
		{"phenomenon", "en", "phenomena"},
		{"baby", "en", "babies"}, // consonant before 'y'
		{"toy", "en", "toys"},    // vowel before 'y'
		{"hero", "en", "heroes"},
		{"dish", "en", "dishes"},
		{"watch", "en", "watches"},
		{"box", "en", "boxes"},
		{"bus", "en", "buses"},
		{"cat", "en", "cats"},  // default case
		{"chat", "fr", "chat"}, // unsupported language
	}

	for _, test := range tests {
		t.Run(test.word, func(t *testing.T) {
			actual := Pluralize(test.word, test.language)
			require.Equal(t, test.expected, actual, "input: %s", test.word)
		})
	}
}
