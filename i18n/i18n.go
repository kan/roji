package i18n

import (
	"encoding/json"
	"fmt"
	"sync"
)

var (
	currentLang string
	msgs        map[string]map[string]string // lang -> key -> value
	once        sync.Once
)

func init() {
	msgs = make(map[string]map[string]string)
	// Always load English at package init so T() works before Init()
	loadLang("en")
	currentLang = "en"
}

func loadLang(lang string) {
	if _, ok := msgs[lang]; ok {
		return
	}
	data, err := messagesFS.ReadFile("messages/" + lang + ".json")
	if err != nil {
		return
	}
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	msgs[lang] = m
}

// Init detects the CLI language from environment variables and loads messages.
// Safe to call multiple times; detection runs only once.
func Init() {
	once.Do(func() {
		lang := DetectCLI()
		if lang != "en" {
			loadLang(lang)
		}
		currentLang = lang
	})
}

// SetLang explicitly sets the active language ("en", "ja").
func SetLang(lang string) {
	loadLang(lang)
	currentLang = lang
}

// Lang returns the current language code.
func Lang() string {
	return currentLang
}

// T returns the translated string for the given key.
// Fallback order: current language -> "en" -> key itself.
func T(key string) string {
	if m, ok := msgs[currentLang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if currentLang != "en" {
		if m, ok := msgs["en"]; ok {
			if v, ok := m[key]; ok {
				return v
			}
		}
	}
	return key
}

// Tf returns a formatted translated string using fmt.Sprintf.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}

// Messages returns the full message map for the given language.
// Used to inject translations into dashboard templates as JSON.
func Messages(lang string) map[string]string {
	loadLang(lang)
	if m, ok := msgs[lang]; ok {
		return m
	}
	return msgs["en"]
}

// AllMessages returns message maps for all supported languages.
// Used to send both languages to the dashboard for client-side switching.
func AllMessages() map[string]map[string]string {
	loadLang("en")
	loadLang("ja")
	return map[string]map[string]string{"en": msgs["en"], "ja": msgs["ja"]}
}
