package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCreditsCommand(t *testing.T) {
	var out bytes.Buffer
	creditsCmd.SetOut(&out)
	t.Cleanup(func() { creditsCmd.SetOut(nil) })
	creditsCmd.Run(creditsCmd, nil)
	got := out.String()

	// roji's own license comes first, then CREDITS, then THIRD_PARTY_NOTICES
	wants := []string{
		"Copyright (c) 2025 kan",
		"github.com/spf13/cobra",
		"Petite Vue",
	}
	last := -1
	for _, want := range wants {
		i := strings.Index(got, want)
		if i < 0 {
			t.Fatalf("output lacks %q", want)
		}
		if i < last {
			t.Errorf("%q appears out of order", want)
		}
		last = i
	}
}
