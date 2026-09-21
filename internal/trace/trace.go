// Package trace provides ephemeral, allowlisted execution metadata for CLI diagnostics.
package trace

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"
)

//nolint:revive
const Schema = "jeq.trace.v1"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)

//nolint:revive
type Observer interface {
	Attempt(attempt, status int)
	Retrying(attempt, budget, status int)
}

//nolint:revive
type Config struct {
	Enabled          bool
	Command, ID      string
	Out              io.Writer
	mu               sync.Mutex
	sequence, events uint64
	suppressed       bool
}
type event struct {
	Schema         string   `json:"schema"`
	Sequence       uint64   `json:"sequence"`
	TraceID        string   `json:"trace_id,omitempty"`
	EntryPoint     string   `json:"entry_point"`
	Command        string   `json:"command"`
	Event          string   `json:"event"`
	Phase          string   `json:"phase,omitempty"`
	Outcome        string   `json:"outcome,omitempty"`
	ErrorCode      string   `json:"error_code,omitempty"`
	OperationIndex int      `json:"operation_index,omitempty"`
	Total          int      `json:"total,omitempty"`
	Attempt        int      `json:"attempt,omitempty"`
	AttemptBudget  int      `json:"attempt_budget,omitempty"`
	HTTPStatus     int      `json:"http_status,omitempty"`
	HTTPClass      int      `json:"http_status_class,omitempty"`
	Seen           int      `json:"seen,omitempty"`
	Succeeded      int      `json:"succeeded,omitempty"`
	Emitted        int      `json:"emitted,omitempty"`
	Failed         int      `json:"failed,omitempty"`
	Model          string   `json:"model,omitempty"`
	ModelSource    string   `json:"model_source,omitempty"`
	Framing        string   `json:"framing,omitempty"`
	Pointer        string   `json:"pointer,omitempty"`
	QuestionNames  []string `json:"question_names,omitempty"`
	QuestionTypes  []string `json:"question_types,omitempty"`
	Pass           int      `json:"pass"`
	Ambiguous      int      `json:"ambiguous"`
	Reject         int      `json:"reject"`
}
type contextKey struct{}

//nolint:revive
func ValidateID(id string) bool { return id == "" || idPattern.MatchString(id) }

//nolint:revive
func New(enabled bool, id string, out io.Writer) *Config {
	return &Config{Enabled: enabled, ID: id, Out: out}
}

//nolint:revive
func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

//nolint:revive
func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(contextKey{}).(*Config)
	return cfg
}

//nolint:revive
func (c *Config) Emit(command, name, phase, outcome, code string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emit(command, name, phase, outcome, code, 0, 0)
}

//nolint:revive
func (c *Config) Attempt(a, s int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emitHTTP(c.Command, "request.attempted", a, 0, s)
}

//nolint:revive
func (c *Config) Retrying(a, b, s int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emitHTTP(c.Command, "request.retrying", a, b, s)
}

func (c *Config) emitHTTP(command, name string, a, b, s int) {
	if c == nil || !c.Enabled || c.Out == nil || !c.reserve(name) {
		return
	}
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: name, Phase: "transport", Outcome: "started", Attempt: a, AttemptBudget: b, HTTPStatus: s, HTTPClass: s / 100})
}

//nolint:revive
func (c *Config) EmitMetadata(command, model, source, framing, pointer string, names, types []string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c == nil || !c.Enabled || c.Out == nil || !c.reserve("preflight.completed") {
		return
	}
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: "preflight.completed", Phase: "preflight", Outcome: "success", Model: model, ModelSource: source, Framing: framing, Pointer: pointer, QuestionNames: names, QuestionTypes: types})
}

//nolint:revive
func (c *Config) EmitPolicy(command string, pass, ambiguous, reject int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c == nil || !c.Enabled || c.Out == nil || !c.reserve("run.completed") {
		return
	}
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: "run.completed", Phase: "offline_policy", Outcome: "success", Pass: pass, Ambiguous: ambiguous, Reject: reject})
}

//nolint:revive
func (c *Config) EmitSummary(command string, seen, succeeded, emitted, failed int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c == nil || !c.Enabled || c.Out == nil {
		return
	}
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: "run.completed", Phase: "summary", Outcome: "success", Seen: seen, Succeeded: succeeded, Emitted: emitted, Failed: failed})
}

//nolint:revive
func (c *Config) EmitOperation(command, name, phase, outcome, code string, index, total int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if name == "operation.started" {
		if c.events >= 64 {
			if !c.suppressed {
				c.suppressed = true
				c.emitUnbounded(c.Command, "events.suppressed", "trace", "bounded", "")
			}
			return
		}
		c.events++
	}
	c.emit(command, name, phase, outcome, code, index, total)
}

func (c *Config) emit(command, name, phase, outcome, code string, index, total int) {
	if c == nil || !c.Enabled || c.Out == nil || !c.reserve(name) {
		return
	}
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: name, Phase: phase, Outcome: outcome, ErrorCode: code, OperationIndex: index, Total: total})
}

func (c *Config) reserve(name string) bool {
	if name == "run.failed" || name == "run.completed" || name == "events.suppressed" {
		return true
	}
	if c.events < 256 {
		c.events++
		return true
	}
	if !c.suppressed {
		c.suppressed = true
		c.emitUnbounded(c.Command, "events.suppressed", "trace", "bounded", "")
	}
	return false
}

func (c *Config) emitUnbounded(command, name, phase, outcome, code string) {
	c.write(event{Schema: Schema, Sequence: c.nextSequence(), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: name, Phase: phase, Outcome: outcome, ErrorCode: code})
}

func (c *Config) write(e event) {
	b, _ := json.Marshal(e)
	_, _ = io.WriteString(c.Out, string(b)+"\n")
}

//nolint:revive
func ResolveID(flag, env string) (string, bool) {
	id := strings.TrimSpace(flag)
	if id == "" {
		id = strings.TrimSpace(env)
	}
	return id, ValidateID(id)
}

func (c *Config) nextSequence() uint64 { c.sequence++; return c.sequence }
