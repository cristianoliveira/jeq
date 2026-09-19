package cli_test

import (
	"os"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
)

func TestModelPrecedence(t *testing.T) {
	t.Setenv("TYPESAFE_DEFAULT_MODEL", "jev-preview")

	tests := []struct {
		name      string
		flagModel string
		want      string
	}{
		{"flag wins over env", "jev-1.13.0", "jev-1.13.0"},
		{"env wins over default", "", "jev-preview"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cli.ResolveModel(tt.flagModel, os.Getenv); got != tt.want {
				t.Errorf("ResolveModel = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("default applies when neither is set", func(t *testing.T) {
		t.Setenv("TYPESAFE_DEFAULT_MODEL", "")
		if got := cli.ResolveModel("", os.Getenv); got != "jev-latest" {
			t.Errorf("ResolveModel = %q, want jev-latest", got)
		}
	})
}
