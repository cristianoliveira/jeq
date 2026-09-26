package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cristianoliveira/jeq/internal/domain/contract"
	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestResolveConfiguredModelPrecedenceAndSource(t *testing.T) {
	read := func(path string, _ int64) ([]byte, *jeq.Error) {
		if path == "/cfg/config.json" {
			return []byte(`{"default_model":"from-config"}`), nil
		}
		return nil, jeq.NewError(jeq.CodeInputInvalid, "missing")
	}
	getenv := func(key string) string {
		if key == "XDG_CONFIG_HOME" {
			return "/cfg"
		}
		if key == DefaultModelEnv {
			return "from-env"
		}
		return ""
	}
	for _, tc := range []struct{ name, flag, want, source string }{
		{"flag", "from-flag", "from-flag", "flag"},
		{"environment", "", "from-env", "environment"},
		{"config", "", "from-config", "config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flag := tc.flag
			if tc.name == "config" {
				getenv = func(key string) string {
					if key == "XDG_CONFIG_HOME" {
						return "/cfg"
					}
					return ""
				}
			}
			configPath := ""
			if tc.name == "config" {
				configPath = "/cfg/config.json"
			}
			got, source, err := ResolveConfiguredModelWithSource(flag, configPath, getenv, read, nil)
			require.Nil(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.source, source)
		})
	}
}

func TestConfigUsesXDGThenHomeAndIgnoresMissingDefault(t *testing.T) {
	readOptional := func(path string, _ int64) ([]byte, *jeq.Error, bool) {
		if path == "/home/.config/jeq/config.json" {
			return []byte(`{"default_model":"home-model"}`), nil, true
		}
		return nil, nil, false
	}
	model, source, err := ResolveConfiguredModelWithSource("", "", func(key string) string {
		if key == "HOME" {
			return "/home"
		}
		return ""
	}, nil, readOptional)
	require.Nil(t, err)
	assert.Equal(t, "home-model", model)
	assert.Equal(t, "config", source)
	model, source, err = ResolveConfiguredModelWithSource("", "", func(string) string { return "" }, nil, readOptional)
	require.Nil(t, err)
	assert.Equal(t, DefaultModel, model)
	assert.Equal(t, "default", source)
}

func TestExplicitConfigIsStrictAndMissingIsAnError(t *testing.T) {
	t.Run("missing explicit config is rejected", func(t *testing.T) {
		read := func(string, int64) ([]byte, *jeq.Error) {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "source missing.json: does not exist")
		}
		_, err := ResolveConfiguredModel("", "missing.json", func(string) string { return "" }, read)
		require.NotNil(t, err, "missing explicit config must fail")
	})

	t.Run("unknown explicit config fields are rejected", func(t *testing.T) {
		read := func(string, int64) ([]byte, *jeq.Error) {
			return []byte(`{"default_model":"m","credentials":"secret"}`), nil
		}
		_, err := ResolveConfiguredModel("", "config.json", func(string) string { return "" }, read)
		require.NotNil(t, err, "unknown config fields must fail")
	})
}

func TestSharedQuestionSourceAcceptsFileOrInlineButNotBoth(t *testing.T) {
	for _, tc := range []struct {
		name     string
		source   questionSourceFlags
		wantCode jeq.Code
	}{
		{"both", questionSourceFlags{fileSet: true, inlineSet: true}, jeq.CodeSourceConflict},
		{"neither", questionSourceFlags{}, jeq.CodeSourceConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkQuestionSource(tc.source)
			require.NotNil(t, err)
			assert.Equal(t, tc.wantCode, err.Code)
		})
	}
	deps := AskDeps{ReadFile: func(string, int64) ([]byte, *jeq.Error) {
		return []byte(`{"questions":{"q":{"type":"noul","instructions":"ok"}}}`), nil
	}}
	questions, _, err := readQuestionSource(deps, questionSourceFlags{file: "q.json", fileSet: true})
	require.Nil(t, err)
	assert.Equal(t, contract.TypeNoul, questions["q"].Type)
	inline, _, err := readQuestionSource(deps, questionSourceFlags{inline: `{"questions":{"q":{"type":"noul","instructions":"ok"}}}`, inlineSet: true})
	require.Nil(t, err)
	assert.Equal(t, contract.TypeNoul, inline["q"].Type)
}

type reduceTestClient struct {
	calls   int
	request contract.Request
}

func (c *reduceTestClient) Evaluate(_ context.Context, request contract.Request) (contract.Response, *jeq.Error) {
	c.calls++
	c.request = request
	return contract.Response{Model: "m", Answers: map[string]contract.Answer{}, Usage: contract.Usage{}, Extra: map[string]json.RawMessage{"server_extra": json.RawMessage(`"kept"`)}}, nil
}

func TestReduceJSONAndNDJSONMakeOneOrderedRequest(t *testing.T) {
	for _, framing := range []string{"json", "ndjson"} {
		t.Run(framing, func(t *testing.T) {
			input := `[ {"n":1}, {"n":2} ]`
			if framing == "ndjson" {
				input = "{\"n\":1}\n{\"n\":2}\n"
			}
			client := &reduceTestClient{}
			deps := reduceDeps(client, input)
			var out, stderr bytes.Buffer
			args := []string{"reduce", "--as", "summary", "--input", framing, "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"summarize"}}}`, "--model", "reduce-model"}
			code := RunWithDeps(args, &out, &stderr, streamRenderer{}, deps)
			require.Equal(t, 0, code, "stdout=%q stderr=%q", out.String(), stderr.String())
			compact := strings.ReplaceAll(out.String(), " ", "")
			assert.Equal(t, 1, client.calls)
			assert.Equal(t, "reduce-model", client.request.Model)
			assert.Equal(t, `[{"n":1},{"n":2}]`, string(bytes.ReplaceAll(client.request.State, []byte(" "), nil)))
			assert.Contains(t, compact, `"items":[{"n":1},{"n":2}]`)
			assert.Contains(t, compact, `"server_extra":"kept"`)
		})
	}
}

func TestMapAndReduceAcceptInlineQuestionSource(t *testing.T) {
	client := &reduceTestClient{}
	deps := reduceDeps(client, `{"state":"s"}`)
	var out, stderr bytes.Buffer
	mapCode := RunWithDeps([]string{"map", "--as", "mapped", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`, "--model", "m"}, &out, &stderr, streamRenderer{}, deps)
	require.Equal(t, 0, mapCode, "map output=%q", out.String())
	reduceCode := RunWithDeps([]string{"reduce", "--as", "reduced", "--input", "ndjson", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`, "--model", "m"}, &out, &stderr, streamRenderer{}, deps)
	require.Equal(t, 0, reduceCode, "reduce output=%q", out.String())
	assert.Equal(t, 2, client.calls)
}

func TestReduceInvalidConfigDoesNotCreateClient(t *testing.T) {
	client := &reduceTestClient{}
	deps := reduceDeps(client, `[{"state":"s"}]`)
	deps.ReadOptionalFile = func(string, int64) ([]byte, *jeq.Error, bool) {
		return []byte(`{"default_model":"m","extra":true}`), nil, true
	}
	deps.Getenv = func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "key"
		}
		if key == "XDG_CONFIG_HOME" {
			return "/cfg"
		}
		return ""
	}
	var out, stderr bytes.Buffer
	code := RunWithDeps([]string{"reduce", "--as", "r", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`}, &out, &stderr, streamRenderer{}, deps)
	assert.Equal(t, 2, code)
	assert.Zero(t, client.calls)
}

func reduceDeps(client APIClient, input string) AskDeps {
	stdin := strings.NewReader(input)
	return AskDeps{Stdin: stdin, ReadStdin: func(_ io.Reader, _ int64, _ bool) ([]byte, *jeq.Error) { return []byte(input), nil }, ReadFile: func(string, int64) ([]byte, *jeq.Error) {
		return nil, jeq.NewError(jeq.CodeInputInvalid, "unexpected file read")
	}, Getenv: func(key string) string {
		if key == "TYPESAFE_API_KEY" {
			return "key"
		}
		return ""
	}, NewClient: func(string, time.Duration, string, int, func(string)) APIClient { return client }}
}
