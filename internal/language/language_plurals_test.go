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
		{"chat", "GG", "chat"}, // unsupported language
	}

	for _, test := range tests {
		t.Run(test.word, func(t *testing.T) {
			actual := Pluralize(test.word, test.language)
			require.Equal(t, test.expected, actual, "case %s=>%s (LANG=%s):", test.word, test.expected, test.language)
		})
	}
}

func TestSingularize(t *testing.T) {
	cases := []struct {
		word     string
		language string
		expected string
	}{
		// 1) -ies → -y
		{"babies", "en", "baby"},
		{"puppies", "en", "puppy"},

		// 2) -ves → -f / -fe
		{"knives", "en", "knife"},
		{"wolves", "en", "wolf"},

		// 3) -yses → -ysis
		{"analyses", "en", "analysis"},
		{"paralyses", "en", "paralysis"},

		// 4a) -oes → -o
		{"heroes", "en", "hero"},
		{"potatoes", "en", "potato"},

		// 4b) -shes → -sh
		{"dishes", "en", "dish"},
		{"wishes", "en", "wish"},

		// 4c) -ches → -ch
		{"watches", "en", "watch"},
		{"branches", "en", "branch"},

		// 4d) -xes → -x
		{"boxes", "en", "box"},
		{"foxes", "en", "fox"},

		// 4e) -ses → -s
		{"buses", "en", "bus"},
		{"glasses", "en", "glass"},

		// 5) simple -s removal
		{"cats", "en", "cat"},
		{"dogs", "en", "dog"},

		// 6) unsupported language
		{"dogs", "GG", "dogs"},
		{"dogs", "GG", "dogs"},
	}

	for _, c := range cases {
		t.Run(c.word, func(t *testing.T) {
			got := Singularize(c.word, c.language)
			require.Equal(t, c.expected, got, "case %s=>%s (LANG=%s):", c.word, c.expected, c.language)
		})
	}
}
