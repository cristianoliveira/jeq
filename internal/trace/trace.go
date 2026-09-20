// Package trace provides ephemeral, allowlisted execution metadata for CLI diagnostics.
package trace

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync/atomic"
)

// Schema identifies the trace wire format.
const Schema = "jeq.trace.v1"

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)

// Observer receives typed transport lifecycle metadata.
type Observer interface {
	Attempt(attempt, status int)
	Retrying(attempt, maxRetries, status int)
}

// Config struct controls one ephemeral trace sink.
type Config struct {
	Enabled    bool
	Command    string
	ID         string
	Out        io.Writer
	sequence   uint64
	detailed   uint64
	suppressed bool
}
type event struct {
	Schema         string `json:"schema"`
	Sequence       uint64 `json:"sequence"`
	TraceID        string `json:"trace_id,omitempty"`
	EntryPoint     string `json:"entry_point"`
	Command        string `json:"command"`
	Event          string `json:"event"`
	Phase          string `json:"phase,omitempty"`
	Outcome        string `json:"outcome,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	OperationIndex int    `json:"operation_index,omitempty"`
	Total          int    `json:"total,omitempty"`
	Attempt        int    `json:"attempt,omitempty"`
	AttemptBudget  int    `json:"attempt_budget,omitempty"`
	HTTPStatus     int    `json:"http_status,omitempty"`
	HTTPClass      int    `json:"http_status_class,omitempty"`
}
type contextKey struct{}

// ValidateID checks the documented correlation identifier grammar.
func ValidateID(id string) bool { return id == "" || idPattern.MatchString(id) }

// New builds an ephemeral trace configuration.
func New(enabled bool, id string, out io.Writer) *Config {
	return &Config{Enabled: enabled, ID: id, Out: out}
}

// WithContext attaches tracing to a command context.
func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext retrieves tracing from a command context.
func FromContext(ctx context.Context) *Config {
	cfg, _ := ctx.Value(contextKey{}).(*Config)
	return cfg
}

// Emit writes one allowlisted event.
func (c *Config) Emit(command, name, phase, outcome, code string) {
	c.emit(command, name, phase, outcome, code, 0, 0)
}

// Attempt records one HTTP attempt.
func (c *Config) Attempt(attempt, status int) {
	c.emitHTTP(c.Command, "request.attempted", attempt, 0, status)
}

// Retrying records one retry decision.
func (c *Config) Retrying(attempt, budget, status int) {
	c.emitHTTP(c.Command, "request.retrying", attempt, budget, status)
}

func (c *Config) emitHTTP(command, name string, attempt, budget, status int) {
	if c == nil || !c.Enabled || c.Out == nil {
		return
	}
	e := event{Schema: Schema, Sequence: atomic.AddUint64(&c.sequence, 1), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: name, Phase: "transport", Outcome: "started", Attempt: attempt, AttemptBudget: budget, HTTPStatus: status, HTTPClass: status / 100}
	b, _ := json.Marshal(e)
	_, _ = io.WriteString(c.Out, string(b)+"\n")
}

// EmitOperation records one bounded operation event.
func (c *Config) EmitOperation(command, name, phase, outcome, code string, index, total int) {
	if name == "operation.started" {
		if c.detailed >= 64 {
			if !c.suppressed {
				c.suppressed = true
				c.Emit(command, "events.suppressed", "trace", "bounded", "")
			}
			return
		}
		c.detailed++
	}
	c.emit(command, name, phase, outcome, code, index, total)
}

func (c *Config) emit(command, name, phase, outcome, code string, index, total int) {
	if c == nil || !c.Enabled || c.Out == nil {
		return
	}
	e := event{Schema: Schema, Sequence: atomic.AddUint64(&c.sequence, 1), TraceID: c.ID, EntryPoint: "cli", Command: command, Event: name, Phase: phase, Outcome: outcome, ErrorCode: code, OperationIndex: index, Total: total}
	b, _ := json.Marshal(e)
	_, _ = io.WriteString(c.Out, string(b)+"\n")
}

// ResolveID applies flag-over-environment precedence.
func ResolveID(flag, env string) (string, bool) {
	id := strings.TrimSpace(flag)
	if id == "" {
		id = strings.TrimSpace(env)
	}
	return id, ValidateID(id)
}
