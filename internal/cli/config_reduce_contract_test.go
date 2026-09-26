package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

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
			if err != nil || got != tc.want || source != tc.source {
				t.Fatalf("got=%q source=%q err=%v", got, source, err)
			}
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
	if err != nil || model != "home-model" || source != "config" {
		t.Fatalf("model=%q source=%q err=%v", model, source, err)
	}
	model, source, err = ResolveConfiguredModelWithSource("", "", func(string) string { return "" }, nil, readOptional)
	if err != nil || model != DefaultModel || source != "default" {
		t.Fatalf("missing model=%q source=%q err=%v", model, source, err)
	}
}

func TestExplicitConfigIsStrictAndMissingIsAnError(t *testing.T) {
	t.Run("missing explicit config is rejected", func(t *testing.T) {
		read := func(string, int64) ([]byte, *jeq.Error) {
			return nil, jeq.NewError(jeq.CodeInputInvalid, "source missing.json: does not exist")
		}
		if _, err := ResolveConfiguredModel("", "missing.json", func(string) string { return "" }, read); err == nil {
			t.Fatal("missing explicit config must fail")
		}
	})

	t.Run("unknown explicit config fields are rejected", func(t *testing.T) {
		read := func(string, int64) ([]byte, *jeq.Error) {
			return []byte(`{"default_model":"m","credentials":"secret"}`), nil
		}
		if _, err := ResolveConfiguredModel("", "config.json", func(string) string { return "" }, read); err == nil {
			t.Fatal("unknown config fields must fail")
		}
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
			if err := checkQuestionSource(tc.source); err == nil || err.Code != tc.wantCode {
				t.Fatalf("err=%v", err)
			}
		})
	}
	deps := AskDeps{ReadFile: func(string, int64) ([]byte, *jeq.Error) {
		return []byte(`{"questions":{"q":{"type":"noul","instructions":"ok"}}}`), nil
	}}
	questions, _, err := readQuestionSource(deps, questionSourceFlags{file: "q.json", fileSet: true})
	if err != nil || questions["q"].Type != contract.TypeNoul {
		t.Fatalf("questions=%v err=%v", questions, err)
	}
	inline, _, err := readQuestionSource(deps, questionSourceFlags{inline: `{"questions":{"q":{"type":"noul","instructions":"ok"}}}`, inlineSet: true})
	if err != nil || inline["q"].Type != contract.TypeNoul {
		t.Fatalf("inline=%v err=%v", inline, err)
	}
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
			if code := RunWithDeps(args, &out, &stderr, streamRenderer{}, deps); code != 0 {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
			}
			compact := strings.ReplaceAll(out.String(), " ", "")
			if client.calls != 1 || client.request.Model != "reduce-model" || string(bytes.ReplaceAll(client.request.State, []byte(" "), nil)) != `[{"n":1},{"n":2}]` {
				t.Fatalf("calls=%d model=%q state=%s", client.calls, client.request.Model, client.request.State)
			}
			if !strings.Contains(compact, `"items":[{"n":1},{"n":2}]`) || !strings.Contains(compact, `"server_extra":"kept"`) {
				t.Fatalf("out=%q", out.String())
			}
		})
	}
}

func TestMapAndReduceAcceptInlineQuestionSource(t *testing.T) {
	client := &reduceTestClient{}
	deps := reduceDeps(client, `{"state":"s"}`)
	var out, stderr bytes.Buffer
	if code := RunWithDeps([]string{"map", "--as", "mapped", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`, "--model", "m"}, &out, &stderr, streamRenderer{}, deps); code != 0 {
		t.Fatalf("map code=%d out=%q", code, out.String())
	}
	if code := RunWithDeps([]string{"reduce", "--as", "reduced", "--input", "ndjson", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`, "--model", "m"}, &out, &stderr, streamRenderer{}, deps); code != 0 {
		t.Fatalf("reduce code=%d out=%q", code, out.String())
	}
	if client.calls != 2 {
		t.Fatalf("calls=%d", client.calls)
	}
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
	if code := RunWithDeps([]string{"reduce", "--as", "r", "--questions-json", `{"questions":{"q":{"type":"noul","instructions":"judge"}}}`}, &out, &stderr, streamRenderer{}, deps); code != 2 || client.calls != 0 {
		t.Fatalf("code=%d calls=%d out=%q", code, client.calls, out.String())
	}
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
