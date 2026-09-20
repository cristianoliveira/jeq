package cli_test

import (
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestResolveBaseURLUsesOnlyEnvironmentAndValidatesHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        string
		fail        bool
	}{{"default", "", cli.DefaultBaseURL, false}, {"http", "http://localhost:8080/", "http://localhost:8080", false}, {"ftp", "ftp://host", "", true}, {"relative", "not-a-url", "", true}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cli.ResolveBaseURL(func(string) string { return tc.value })
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}

func TestExplicitConfigIsValidatedBeforeModelPrecedence(t *testing.T) {
	read := func(string, int64) ([]byte, *jeq.Error) { return []byte(`{"unsupported":true}`), nil }
	_, _, err := cli.ResolveConfiguredModelWithSource("flag-model", "/explicit.json", func(key string) string {
		if key == "JEQ_CONFIG" {
			return "/explicit.json"
		}
		return ""
	}, read, nil)
	if err == nil {
		t.Fatal("invalid explicit config must fail even when flag model wins")
	}
}
