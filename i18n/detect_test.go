package i18n

import (
	"os"
	"testing"
)

func TestDetectCLI_Default(t *testing.T) {
	// Clear all locale env vars
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		os.Unsetenv(env)
	}

	got := DetectCLI()
	if got != "en" {
		t.Errorf("DetectCLI() with no env = %q, want \"en\"", got)
	}
}

func TestDetectCLI_Japanese(t *testing.T) {
	tests := []struct {
		env   string
		value string
		want  string
	}{
		{"LANG", "ja_JP.UTF-8", "ja"},
		{"LANG", "ja_JP.eucJP", "ja"},
		{"LANG", "ja", "ja"},
		{"LC_MESSAGES", "ja_JP.UTF-8", "ja"},
		{"LC_ALL", "ja_JP.UTF-8", "ja"},
	}

	for _, tt := range tests {
		t.Run(tt.env+"="+tt.value, func(t *testing.T) {
			// Clear all
			for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
				os.Unsetenv(env)
			}
			os.Setenv(tt.env, tt.value)
			defer os.Unsetenv(tt.env)

			got := DetectCLI()
			if got != tt.want {
				t.Errorf("DetectCLI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectCLI_English(t *testing.T) {
	tests := []struct {
		env   string
		value string
	}{
		{"LANG", "en_US.UTF-8"},
		{"LANG", "C"},
		{"LANG", "POSIX"},
		{"LANG", "fr_FR.UTF-8"},
	}

	for _, tt := range tests {
		t.Run(tt.env+"="+tt.value, func(t *testing.T) {
			for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
				os.Unsetenv(env)
			}
			os.Setenv(tt.env, tt.value)
			defer os.Unsetenv(tt.env)

			got := DetectCLI()
			if got != "en" {
				t.Errorf("DetectCLI() = %q, want \"en\"", got)
			}
		})
	}
}

func TestDetectCLI_Priority(t *testing.T) {
	// LC_ALL has highest priority
	os.Setenv("LANG", "ja_JP.UTF-8")
	os.Setenv("LC_MESSAGES", "ja_JP.UTF-8")
	os.Setenv("LC_ALL", "en_US.UTF-8")
	defer func() {
		os.Unsetenv("LANG")
		os.Unsetenv("LC_MESSAGES")
		os.Unsetenv("LC_ALL")
	}()

	got := DetectCLI()
	if got != "en" {
		t.Errorf("DetectCLI() with LC_ALL=en = %q, want \"en\"", got)
	}
}

func TestDetectHTTP(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"", "en"},
		{"en-US,en;q=0.9", "en"},
		{"ja", "ja"},
		{"ja-JP,ja;q=0.9,en;q=0.8", "ja"},
		{"en-US,en;q=0.9,ja;q=0.8", "en"},
		{"fr-FR,fr;q=0.9", "en"},    // unsupported -> English
		{"ja;q=0.5,en;q=0.9", "en"}, // English has higher quality
		{"ja;q=0.9,en;q=0.5", "ja"}, // Japanese has higher quality
		{"*", "en"},                 // wildcard -> default (English)
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := DetectHTTP(tt.header)
			if got != tt.want {
				t.Errorf("DetectHTTP(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
