package i18n

import (
	"os"
	"strings"

	"golang.org/x/text/language"
)

var supported = []language.Tag{
	language.English,
	language.Japanese,
}

var matcher = language.NewMatcher(supported)

// DetectCLI detects language from environment variables (LANG, LC_ALL, LC_MESSAGES).
// Returns "ja" for Japanese, "en" for everything else.
func DetectCLI() string {
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		val := os.Getenv(env)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		if strings.HasPrefix(val, "ja") {
			return "ja"
		}
		// Any non-empty, non-Japanese locale → English
		return "en"
	}
	return "en"
}

// DetectHTTP detects language from an Accept-Language header value.
// Returns "ja" for Japanese, "en" for everything else.
func DetectHTTP(acceptLang string) string {
	if acceptLang == "" {
		return "en"
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLang)
	if err != nil || len(tags) == 0 {
		return "en"
	}
	_, idx, _ := matcher.Match(tags...)
	if idx == 1 { // Japanese
		return "ja"
	}
	return "en"
}
