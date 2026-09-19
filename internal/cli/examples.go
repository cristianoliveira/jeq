package cli

import (
	"fmt"
	"strings"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/spf13/cobra"
)

type exampleSummary struct {
	ID      string   `json:"id"`
	Purpose string   `json:"purpose"`
	Covers  []string `json:"covers"`
	Cost    string   `json:"network_calls"`
}

type exampleRecipe struct {
	ID           string   `json:"id"`
	Purpose      string   `json:"purpose"`
	Covers       []string `json:"covers"`
	Requirements []string `json:"requirements"`
	Cost         string   `json:"network_calls"`
	Shell        string   `json:"shell"`
	InputShape   string   `json:"input_shape"`
	OutputShape  string   `json:"output_shape"`
	Privacy      string   `json:"privacy"`
	Exits        string   `json:"exit_semantics,omitempty"`
	NextStep     string   `json:"next_step"`
}

type examplesCatalog struct {
	Examples []exampleSummary `json:"examples"`
	NextStep string           `json:"next_step"`
}

var exampleRecipes = []exampleRecipe{
	{
		ID: "ask-native", Purpose: "Send one native request from stdin.", Covers: []string{"ask"},
		Requirements: []string{"installed gev", "TYPESAFE_API_KEY"}, Cost: "1 API request",
		Shell: `GEV_BIN=${GEV_BIN:-gev}
set -euo pipefail
printf '%s\n' '{"model":"jev-latest","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}' |
  "$GEV_BIN" ask --request -`,
		InputShape: "native request JSON on stdin: {model,state,questions}", OutputShape: "one response envelope with _gev-free answers and usage",
		Privacy: "stdin state is sent to TypeSafe; project response fields before logs.", NextStep: "run gev examples map-gate",
	},
	{
		ID: "map-gate", Purpose: "Judge each record and apply an offline threshold.", Covers: []string{"map", "gate"},
		Requirements: []string{"installed gev", "TYPESAFE_API_KEY"}, Cost: "N API requests for N records; gate is offline",
		Shell: `GEV_BIN=${GEV_BIN:-gev}
set -euo pipefail
printf '%s\n' '{"change":"small"}' '{"change":"large"}' |
  "$GEV_BIN" map --as risk --input ndjson --state-pointer /change \
    --questions-json '{"questions":{"risk":{"type":"noul","instructions":"Is this low risk?"}}}' |
  "$GEV_BIN" gate --as policy --value-pointer /_gev/risk/answers/risk/noul \
    --pass-min 0.80 --reject-max 0.40`,
		InputShape: "NDJSON records containing change", OutputShape: "each record retains state and gains _gev/risk and _gev/policy",
		Privacy: "map sends each selected record; delete sensitive state before logging.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain.", NextStep: "run gev examples reduce-gate",
	},
	{
		ID: "reduce-gate", Purpose: "Judge one complete collection and gate its aggregate signal.", Covers: []string{"reduce", "gate"},
		Requirements: []string{"installed gev", "TYPESAFE_API_KEY"}, Cost: "1 API request for the complete collection; gate is offline",
		Shell: `GEV_BIN=${GEV_BIN:-gev}
set -euo pipefail
printf '%s\n' '{"id":"a","value":1}' '{"id":"b","value":2}' |
  "$GEV_BIN" reduce --as coherent --input ndjson \
    --questions-json '{"questions":{"coherent":{"type":"noul","instructions":"Is this collection coherent?"}}}' |
  "$GEV_BIN" gate --as policy --value-pointer /_gev/coherent/answers/coherent/noul \
    --pass-min 0.80 --reject-max 0.40`,
		InputShape: "ordered NDJSON records; reduce evaluates the complete array", OutputShape: "aggregate response retains items and adds _gev/coherent and _gev/policy",
		Privacy: "the complete collection is sent in one request; delete .items before sharing.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain.", NextStep: "run gev examples map-reduce-gate",
	},
	{
		ID: "map-reduce-gate", Purpose: "Compose local per-record judgments with one relational aggregate gate.", Covers: []string{"map", "reduce", "gate"},
		Requirements: []string{"installed gev", "TYPESAFE_API_KEY", "jq"}, Cost: "N map requests plus 1 reduce request; jq and gate are offline",
		Shell: `GEV_BIN=${GEV_BIN:-gev}
set -euo pipefail
printf '%s\n' '{"file":{"path":"a","content":"one"}}' '{"file":{"path":"b","content":"two"}}' |
  "$GEV_BIN" map --as local --input ndjson --state-pointer /file \
    --questions-json '{"questions":{"local":{"type":"noul","instructions":"Is this file focused?"}}}' |
  jq -c '{file:.file,local:._gev.local.answers.local.noul}' |
  "$GEV_BIN" reduce --as aggregate --input ndjson \
    --questions-json '{"questions":{"aggregate":{"type":"noul","instructions":"Is this related collection coherent?"}}}' |
  "$GEV_BIN" gate --as policy --value-pointer /_gev/aggregate/answers/aggregate/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.items)'`,
		InputShape: "NDJSON records with file:{path,content}", OutputShape: "source-free aggregate/gate evidence after del(.items)",
		Privacy: "source crosses map and reduce; final jq removes .items before logs or sharing.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain; pipefail preserves it.", NextStep: "run gev examples ask-native",
	},
}

// NewExamplesCmd creates the offline workflow recipe discovery command.
func NewExamplesCmd(deps AskDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples [id]",
		Short: "Discover self-contained workflow recipes",
		Example: `  gev examples
  gev examples map-reduce-gate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			renderer, err := outputRenderer(cmd, deps)
			if err != nil {
				return err
			}
			if len(args) > 1 {
				return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("examples accepts one id; received %q", strings.Join(args, " "))).WithRecovery("choose one of: " + exampleIDs())
			}
			if len(args) == 0 {
				items := make([]exampleSummary, 0, len(exampleRecipes))
				for _, recipe := range exampleRecipes {
					items = append(items, exampleSummary{ID: recipe.ID, Purpose: recipe.Purpose, Covers: recipe.Covers, Cost: recipe.Cost})
				}
				return renderer.RenderValue(cmd.OutOrStdout(), examplesCatalog{Examples: items, NextStep: "run gev examples map-reduce-gate"})
			}
			for _, recipe := range exampleRecipes {
				if recipe.ID == args[0] {
					return renderer.RenderValue(cmd.OutOrStdout(), recipe)
				}
			}
			return gev.NewError(gev.CodeInputInvalid, fmt.Sprintf("unknown example id %q", args[0])).WithRecovery("choose one of: " + exampleIDs())
		},
	}
	return cmd
}

func exampleIDs() string {
	ids := make([]string, 0, len(exampleRecipes))
	for _, recipe := range exampleRecipes {
		ids = append(ids, recipe.ID)
	}
	return strings.Join(ids, ", ")
}
