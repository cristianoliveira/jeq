package cli_test

import (
	"strings"
	"testing"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestResolveProviderPrecedenceAndProfiles(t *testing.T) {
	config := []byte(`{"default_provider":"vercel","providers":{"local":{"base_url":"http://127.0.0.1:8787","default_model":"local","auth":"none"}}}`)
	read := func(path string, _ int64) ([]byte, *jeq.Error) {
		if path != "cfg.json" {
			t.Fatalf("path=%s", path)
		}
		return config, nil
	}
	getenv := func(key string) string {
		switch key {
		case "JEQ_CONFIG":
			return "cfg.json"
		case "JEQ_PROVIDER":
			return "local"
		case "JEQ_DEFAULT_MODEL":
			return ""
		}
		return ""
	}
	got, err := cli.ResolveProvider(getenv, read, nil)
	if err != nil || got.Name != "local" || got.BaseURL != "http://127.0.0.1:8787" || got.Auth != "none" || got.Model != "local" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestResolveProviderVercelUsesDedicatedCredential(t *testing.T) {
	getenv := func(key string) string {
		if key == "JEQ_PROVIDER" {
			return "vercel"
		}
		if key == "AI_GATEWAY_API_KEY" {
			return "gateway"
		}
		return ""
	}
	got, err := cli.ResolveProvider(getenv, nil, nil)
	if err != nil || got.BaseURL != "https://ai-gateway.vercel.sh/typesafe" || got.APIKey != "gateway" || got.Model != "typesafe-ai/jev" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestResolveProviderRejectsRemoteUnauthenticatedHTTP(t *testing.T) {
	getenv := func(key string) string {
		if key == "JEQ_PROVIDER" {
			return "custom"
		}
		if key == "JEQ_BASE_URL" {
			return "http://example.test"
		}
		if key == "JEQ_AUTH" {
			return "none"
		}
		return ""
	}
	_, err := cli.ResolveProvider(getenv, nil, nil)
	if err == nil || !strings.Contains(err.Message, "invalid base_url") {
		t.Fatalf("err=%v", err)
	}
}
