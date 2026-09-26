package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/cli"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestResolveProviderPrecedenceAndProfiles(t *testing.T) {
	config := []byte(`{"default_provider":"vercel","providers":{"local":{"base_url":"http://127.0.0.1:8787","default_model":"local","auth":"none"}}}`)
	var gotPath string
	read := func(path string, _ int64) ([]byte, *jeq.Error) {
		gotPath = path
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
	require.Nil(t, err)
	assert.Equal(t, "cfg.json", gotPath)
	assert.Equal(t, "local", got.Name)
	assert.Equal(t, "http://127.0.0.1:8787", got.BaseURL)
	assert.Equal(t, "none", got.Auth)
	assert.Equal(t, "local", got.Model)
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
	require.Nil(t, err)
	assert.Equal(t, "https://ai-gateway.vercel.sh/typesafe", got.BaseURL)
	assert.Equal(t, "gateway", got.APIKey)
	assert.Equal(t, "typesafe-ai/jev", got.Model)
}

func TestResolveProviderUsesVercelOIDCFallback(t *testing.T) {
	getenv := func(key string) string {
		if key == "JEQ_PROVIDER" {
			return "vercel"
		}
		if key == "VERCEL_OIDC_TOKEN" {
			return "oidc-token"
		}
		return ""
	}
	got, err := cli.ResolveProvider(getenv, nil, nil)
	require.Nil(t, err)
	assert.Equal(t, "oidc-token", got.APIKey)
}

func TestResolveProviderRejectsRemoteUnauthenticatedHTTPAndHTTPS(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
	}{
		{name: "unauthenticated remote HTTP is rejected", baseURL: "http://example.test"},
		{name: "unauthenticated remote HTTPS is rejected", baseURL: "https://example.test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				switch key {
				case "JEQ_PROVIDER":
					return "custom"
				case "JEQ_BASE_URL":
					return tc.baseURL
				case "JEQ_AUTH":
					return "none"
				default:
					return ""
				}
			}
			_, err := cli.ResolveProvider(getenv, nil, nil)
			require.NotNil(t, err)
			assert.Contains(t, err.Message, "invalid base_url")
		})
	}
}
