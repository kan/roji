package i18n

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestT_EnglishDefault(t *testing.T) {
	// Reset to English
	SetLang("en")

	got := T("cmd.root.short")
	want := "Reverse proxy for local development"
	if got != want {
		t.Errorf("T(\"cmd.root.short\") = %q, want %q", got, want)
	}
}

func TestT_Japanese(t *testing.T) {
	SetLang("ja")
	defer SetLang("en")

	got := T("cmd.root.short")
	want := "ローカル開発用リバースプロキシ"
	if got != want {
		t.Errorf("T(\"cmd.root.short\") = %q, want %q", got, want)
	}
}

func TestT_FallbackToEnglish(t *testing.T) {
	SetLang("ja")
	defer SetLang("en")

	// Key that might only be in English — test fallback
	// Use a key that we know exists in en.json
	got := T("cmd.health.healthy")
	// Should return Japanese version if it exists, English otherwise
	if got == "cmd.health.healthy" {
		t.Errorf("T should not return the key itself for a known key")
	}
}

func TestT_FallbackToKey(t *testing.T) {
	SetLang("en")

	got := T("nonexistent.key.that.does.not.exist")
	want := "nonexistent.key.that.does.not.exist"
	if got != want {
		t.Errorf("T(unknown key) = %q, want %q (the key itself)", got, want)
	}
}

func TestTf_Interpolation(t *testing.T) {
	SetLang("en")

	got := Tf("cmd.version.output", "1.0.0")
	want := "roji version 1.0.0"
	if got != want {
		t.Errorf("Tf(\"cmd.version.output\", \"1.0.0\") = %q, want %q", got, want)
	}
}

func TestTf_MultipleArgs(t *testing.T) {
	SetLang("en")

	got := Tf("cmd.doctor.summary.errors", 2, 1)
	want := "2 error(s), 1 warning(s)"
	if got != want {
		t.Errorf("Tf = %q, want %q", got, want)
	}
}

func TestLang(t *testing.T) {
	SetLang("en")
	if Lang() != "en" {
		t.Errorf("Lang() = %q, want \"en\"", Lang())
	}

	SetLang("ja")
	if Lang() != "ja" {
		t.Errorf("Lang() = %q, want \"ja\"", Lang())
	}

	SetLang("en") // restore
}

func TestMessages(t *testing.T) {
	m := Messages("en")
	if m == nil {
		t.Fatal("Messages(\"en\") returned nil")
	}
	if _, ok := m["cmd.root.short"]; !ok {
		t.Error("Messages(\"en\") missing key \"cmd.root.short\"")
	}
}

func TestMessages_Japanese(t *testing.T) {
	m := Messages("ja")
	if m == nil {
		t.Fatal("Messages(\"ja\") returned nil")
	}
	if _, ok := m["cmd.root.short"]; !ok {
		t.Error("Messages(\"ja\") missing key \"cmd.root.short\"")
	}
}

func TestMessages_UnknownLang(t *testing.T) {
	m := Messages("fr")
	// Should return English messages as fallback
	if m == nil {
		t.Fatal("Messages(\"fr\") returned nil, expected English fallback")
	}
	if v, ok := m["cmd.root.short"]; !ok || v != "Reverse proxy for local development" {
		t.Error("Messages(\"fr\") should fall back to English")
	}
}

func TestSetLang_UnknownLanguage(t *testing.T) {
	SetLang("fr")
	// T should fall back to English
	got := T("cmd.root.short")
	want := "Reverse proxy for local development"
	if got != want {
		t.Errorf("After SetLang(\"fr\"), T should fall back to English, got %q", got)
	}
	SetLang("en") // restore
}

// TestMessageFiles_NoDuplicateKeys guards the message files against a key
// being defined twice. encoding/json keeps the last one, so a duplicate is
// invisible at runtime while silently discarding whichever text came first.
func TestMessageFiles_NoDuplicateKeys(t *testing.T) {
	for _, lang := range messageLangs(t) {
		t.Run(lang, func(t *testing.T) {
			data, err := messagesFS.ReadFile("messages/" + lang + ".json")
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			dec := json.NewDecoder(bytes.NewReader(data))
			if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
				t.Fatalf("expected a JSON object, got %v (%v)", tok, err)
			}

			seen := map[string]bool{}
			for dec.More() {
				tok, err := dec.Token()
				if err != nil {
					t.Fatalf("read key: %v", err)
				}
				key, ok := tok.(string)
				if !ok {
					t.Fatalf("expected a string key, got %T", tok)
				}
				if seen[key] {
					t.Errorf("duplicate key %q", key)
				}
				seen[key] = true

				if _, err := dec.Token(); err != nil {
					t.Fatalf("read value for %q: %v", key, err)
				}
			}
		})
	}
}

// TestMessageFiles_SameKeys keeps the translations in step: a key present in
// one file and missing from another falls back silently, so the gap only shows
// up as English text in a Japanese session.
//
// English is the reference because it is what every other language falls back
// to. Languages come from the embedded files, so adding one is covered without
// touching this test.
func TestMessageFiles_SameKeys(t *testing.T) {
	const reference = "en"
	refMsgs := Messages(reference)
	if len(refMsgs) == 0 {
		t.Fatalf("no messages for the reference language %q", reference)
	}

	for _, lang := range messageLangs(t) {
		if lang == reference {
			continue
		}
		t.Run(lang, func(t *testing.T) {
			msgs := Messages(lang)
			for key := range refMsgs {
				if _, ok := msgs[key]; !ok {
					t.Errorf("key %q is in %s.json but missing from %s.json", key, reference, lang)
				}
			}
			for key := range msgs {
				if _, ok := refMsgs[key]; !ok {
					t.Errorf("key %q is in %s.json but missing from %s.json", key, lang, reference)
				}
			}
		})
	}
}

// messageLangs returns the language codes of the embedded message files.
func messageLangs(t *testing.T) []string {
	t.Helper()

	entries, err := messagesFS.ReadDir("messages")
	if err != nil {
		t.Fatalf("read messages dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no message files found")
	}

	langs := make([]string, 0, len(entries))
	for _, entry := range entries {
		langs = append(langs, strings.TrimSuffix(entry.Name(), ".json"))
	}
	return langs
}
