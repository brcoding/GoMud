package language

import "strings"

type pluralRule struct {
	suffix         string
	replacement    string
	afterConsonant bool
}

const (
	DEFAULT_SUFFIX = ``
)

var (
	languagePluralRules = map[string][]pluralRule{
		"en": {
			{"fe", "ves", false},         // knife -> knives
			{"f", "ves", false},          // wolf -> wolves
			{"is", "es", false},          // analysis -> analyses
			{"on", "a", false},           // phenomenon -> phenomena
			{"y", "ies", true},           // baby -> babies (handled with consonant condition)
			{"o", "oes", false},          // hero -> heroes
			{"sh", "shes", false},        // dish -> dishes
			{"ch", "ches", false},        // watch -> watches
			{"x", "xes", false},          // box -> boxes
			{"s", "ses", false},          // bus -> buses
			{DEFAULT_SUFFIX, "s", false}, // default to add to end if nothing else matches.
		},
	}
)

// isConsonant checks if a letter is a consonant
func isConsonant(ch byte) bool {
	return !strings.ContainsRune("aeiouAEIOU", rune(ch))
}

func Pluralize(word string, language ...string) string {

	lang := `en`
	if len(language) == 0 {
		lang = language[0]
	}

	pluralReplacements, ok := languagePluralRules[lang]
	if !ok {
		return word
	}

	wordLen := len(word)

	defaultSuffix := ``

	for _, rule := range pluralReplacements {
		// empty suffix is
		if rule.suffix == DEFAULT_SUFFIX {
			defaultSuffix = rule.replacement
			continue
		}
		// See if suffix matches
		if strings.HasSuffix(word, rule.suffix) {
			// Skip if requires the special consonant rule
			if rule.afterConsonant && (wordLen <= 1 || !isConsonant(word[wordLen-2])) {
				continue
			}

			return word[:wordLen-len(rule.suffix)] + rule.replacement
		}
	}

	return word + defaultSuffix
}
