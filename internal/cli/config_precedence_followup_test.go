package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.Nil(t, err)
	assert.Equal(t, "flag-model", model)
	assert.Equal(t, "flag", source)
	model, source, err = ResolveConfiguredModelWithSource("", "", panicEnv, nil, panicReader)
	require.Nil(t, err)
	assert.Equal(t, "env-model", model)
	assert.Equal(t, "environment", source)
}

func TestOptionalConfigIsSkippedWhenHigherModelSourceWins(t *testing.T) {
	reads := 0
	readOptional := func(string, int64) ([]byte, *jeq.Error, bool) {
		reads++
		return nil, nil, false
	}
	model, _, err := ResolveConfiguredModelWithSource("flag-model", "", func(string) string { return "" }, nil, readOptional)
	require.Nil(t, err)
	assert.Equal(t, "flag-model", model)
	assert.Zero(t, reads)
	model, _, err = ResolveConfiguredModelWithSource("", "", func(key string) string {
		if key == DefaultModelEnv {
			return "env-model"
		}
		return ""
	}, nil, readOptional)
	require.Nil(t, err)
	assert.Equal(t, "env-model", model)
	assert.Zero(t, reads)
}

func TestExplicitConfigRejectsMissingUnreadableMalformedWrongTypeEmptyDuplicateAndOversize(t *testing.T) {
	cases := []struct {
		name     string
		document string
	}{
		{name: "malformed JSON is rejected", document: `{"default_model":`},
		{name: "numeric default_model is rejected", document: `{"default_model":1}`},
		{name: "missing default_model is rejected", document: `{}`},
		{name: "duplicate default_model is rejected", document: `{"default_model":"a","default_model":"b"}`},
		{name: "oversized default_model is rejected", document: `{"default_model":"` + strings.Repeat("x", configMaxBytes) + `"}`},
	}
	t.Run("unreadable explicit config is rejected", func(t *testing.T) {
		read := func(string, int64) ([]byte, *jeq.Error) {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "permission denied")
		}
		_, err := ResolveConfiguredModel("", "missing.json", func(string) string { return "" }, read)
		require.NotNil(t, err, "unreadable config accepted")
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			read := func(string, int64) ([]byte, *jeq.Error) { return []byte(tc.document), nil }
			_, err := ResolveConfiguredModel("", "explicit.json", func(string) string { return "" }, read)
			require.NotNil(t, err, "invalid config accepted")
		})
	}
}
