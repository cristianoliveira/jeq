package jeq_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				assert.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, tt.wantCode, err.Code)
		})
	}
}
