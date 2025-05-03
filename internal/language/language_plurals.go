package language

import "strings"

type Pluralizer func(input string) string
type Singularizer func(input string) string

const (
	DEFAULT_SUFFIX = ``
)

var (
	pluralizers = map[string]Pluralizer{
		"en": Pluralizer_en,
	}
	singularizers = map[string]Singularizer{
		"en": Singularizer_en,
	}
)

// isConsonant checks if a letter is a consonant
func isConsonant(ch byte) bool {
	return !strings.ContainsRune("aeiouAEIOU", rune(ch))
}

func Pluralize(word string, language ...string) string {
	lang := `en`
	if len(language) > 0 {
		lang = language[0]
	}

	if f, ok := pluralizers[lang]; ok {
		return f(word)
	}
	return word
}

func Singularize(word string, language ...string) string {
	lang := `en`
	if len(language) > 0 {
		lang = language[0]
	}

	if f, ok := singularizers[lang]; ok {
		return f(word)
	}
	return word
}

func Pluralizer_en(word string) string {

	wordLen := len(word)

	if strings.HasSuffix(word, `y`) {
		if wordLen > 1 && isConsonant(word[wordLen-2]) {
			return word[:wordLen-1] + `ies`
		}
		return word + `s`
	}

	if strings.HasSuffix(word, "fe") {
		return word[:wordLen-2] + "ves" // knife -> knives
	}

	if strings.HasSuffix(word, "f") {
		return word[:wordLen-1] + "ves" // wolf -> wolves
	}

	if strings.HasSuffix(word, "is") {
		return word[:wordLen-2] + "es" // analysis -> analyses
	}

	if strings.HasSuffix(word, "on") {
		return word[:wordLen-2] + "a" // phenomenon -> phenomena
	}

	if strings.HasSuffix(word, "o") {
		return word[:wordLen-1] + "oes" // hero -> heroes
	}

	if strings.HasSuffix(word, "sh") {
		return word + "es" // dish -> dishes
	}

	if strings.HasSuffix(word, "ch") {
		return word + "es" // watch -> watches
	}

	if strings.HasSuffix(word, "x") {
		return word + "es" // box -> boxes
	}

	if strings.HasSuffix(word, "s") {
		return word + "es" // bus -> buses
	}

	return word + `s`
}

func Singularizer_en(word string) string {
	wordLen := len(word)

	// 1) y → ies  (babies → baby)
	if strings.HasSuffix(word, "ies") {
		// only when preceded by a consonant (same test as your pluralizer)
		if wordLen > 3 && isConsonant(word[wordLen-4]) {
			return word[:wordLen-3] + "y"
		}
		// e.g. “dies” → “die” (fallback)
		return word[:wordLen-1]
	}

	// 2) f/fe → ves  (knife → knives, wolf → wolves)
	if strings.HasSuffix(word, "ves") {
		stem := word[:wordLen-3] // chop off "ves"
		if len(stem) > 0 && stem[len(stem)-1] == 'i' {
			// stems like "kni" → drop that "i" + "ife" = "knife"
			return stem[:len(stem)-1] + "ife"
		}
		// e.g. "wol" + "f" = "wolf"
		return stem + "f"
	}

	// 3) is → es  (analysis → analyses)
	//    we catch "-yses" so that we don’t confuse "buses" (→bus) with "analyses" (→analysis)
	if strings.HasSuffix(word, "yses") {
		return word[:wordLen-4] + "ysis"
	}

	// 4) o, sh, ch, x, s + es  (hero→heroes, dish→dishes, watch→watches, box→boxes, bus→buses)
	switch {
	case strings.HasSuffix(word, "oes"),
		strings.HasSuffix(word, "shes"),
		strings.HasSuffix(word, "ches"),
		strings.HasSuffix(word, "xes"),
		strings.HasSuffix(word, "ses"):
		return word[:wordLen-2] // just strip the "es"
	}

	// 5) simple s  (cat→cats)
	if strings.HasSuffix(word, "s") {
		return word[:wordLen-1]
	}

	// nothing to do
	return word
}
