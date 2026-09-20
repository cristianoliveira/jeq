package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestConfiguredModelShortCircuitsLowerSources(t *testing.T) {
	panicReader := func(string, int64) ([]byte, *jeq.Error, bool) { return nil, nil, false }
	panicEnv := func(key string) string {
		if key == DefaultModelEnv {
			return "env-model"
		}
		return ""
	}
	model, source, err := ResolveConfiguredModelWithSource("flag-model", "", panicEnv, nil, panicReader)
	if err != nil || model != "flag-model" || source != "flag" {
		t.Fatalf("model=%q source=%q err=%v", model, source, err)
	}
	model, source, err = ResolveConfiguredModelWithSource("", "", panicEnv, nil, panicReader)
	if err != nil || model != "env-model" || source != "environment" {
		t.Fatalf("model=%q source=%q err=%v", model, source, err)
	}
}

func TestOptionalConfigIsSkippedWhenHigherModelSourceWins(t *testing.T) {
	readOptional := func(string, int64) ([]byte, *jeq.Error, bool) {
		t.Fatal("optional config must not be read")
		return nil, nil, false
	}
	if model, _, err := ResolveConfiguredModelWithSource("flag-model", "", func(string) string { return "" }, nil, readOptional); err != nil || model != "flag-model" {
		t.Fatalf("model=%q err=%v", model, err)
	}
	if model, _, err := ResolveConfiguredModelWithSource("", "", func(key string) string {
		if key == DefaultModelEnv {
			return "env-model"
		}
		return ""
	}, nil, readOptional); err != nil || model != "env-model" {
		t.Fatalf("model=%q err=%v", model, err)
	}
}

func TestExplicitConfigRejectsMissingUnreadableMalformedWrongTypeEmptyDuplicateAndOversize(t *testing.T) {
	cases := []string{
		`{"default_model":`,
		`{"default_model":1}`,
		`{}`,
		`{"default_model":"a","default_model":"b"}`,
		`{"default_model":"` + strings.Repeat("x", configMaxBytes) + `"}`,
	}
	if _, err := ResolveConfiguredModel("", "missing.json", func(string) string { return "" }, func(string, int64) ([]byte, *jeq.Error) {
		return nil, jeq.NewError(jeq.CodeInputInvalid, "permission denied")
	}); err == nil {
		t.Fatal("unreadable config accepted")
	}
	for i, document := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			read := func(string, int64) ([]byte, *jeq.Error) { return []byte(document), nil }
			if _, err := ResolveConfiguredModel("", "explicit.json", func(string) string { return "" }, read); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
