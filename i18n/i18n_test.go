package i18n

import (
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
