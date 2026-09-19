package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianoliveira/gev/internal/domain/gev"
)

func TestConfiguredModelShortCircuitsLowerSources(t *testing.T) {
	panicReader := func(string, int64) ([]byte, *gev.Error, bool) {
		t.Fatal("config must not be read")
		return nil, nil, false
	}
	panicEnv := func(key string) string {
		if key == DefaultModelEnv {
			return "env-model"
		}
		t.Fatalf("unexpected env read: %s", key)
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

func TestExplicitConfigRejectsMalformedWrongTypeEmptyDuplicateAndOversize(t *testing.T) {
	cases := []string{
		`{"default_model":`,
		`{"default_model":1}`,
		`{}`,
		`{"default_model":"a","default_model":"b"}`,
		`{"default_model":"` + strings.Repeat("x", configMaxBytes) + `"}`,
	}
	for i, document := range cases {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			read := func(string, int64) ([]byte, *gev.Error) { return []byte(document), nil }
			if _, err := ResolveConfiguredModel("", "explicit.json", func(string) string { return "" }, read); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
