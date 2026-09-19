package jeq_test

import (
	"testing"

	"github.com/cristianoliveira/jeq/internal/domain/jeq"
)

func TestCheckStdinRejectsDoubleStdin(t *testing.T) {
	tests := []struct {
		name                      string
		forRequest, forQ, forSted bool
		wantCode                  jeq.Code // "" means allowed
	}{
		{name: "stdin for questions only", forQ: true},
		{name: "stdin for request only", forRequest: true},
		{name: "stdin for state only", forSted: true},
		{name: "no stdin at all", wantCode: ""},
		{
			name:     "stdin for questions and state",
			forQ:     true,
			forSted:  true,
			wantCode: jeq.CodeSourceConflict,
		},
		{
			name:       "stdin for request and questions",
			forRequest: true,
			forQ:       true,
			wantCode:   jeq.CodeSourceConflict,
		},
		{
			name:       "stdin for all three",
			forRequest: true,
			forQ:       true,
			forSted:    true,
			wantCode:   jeq.CodeSourceConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jeq.CheckStdin(tt.forRequest, tt.forQ, tt.forSted)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("expected allowed, got %v", err)
				}
				return
			}
			if err == nil || err.Code != tt.wantCode {
				t.Fatalf("expected %q, got %v", tt.wantCode, err)
			}
		})
	}
}
