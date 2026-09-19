package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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
}

var exampleRecipes = []exampleRecipe{
	{
		ID: "validate-native", Purpose: "Validate one native request locally before spending a network call.", Covers: []string{"validate"},
		Requirements: []string{"installed jeq", "bash"}, Cost: "0 API requests; validation is offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '{"model":"jev-latest","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}' |
  "$JEQ_BIN" validate --request -`,
		InputShape: "native request JSON on stdin: {model,state,questions}", OutputShape: "plain validation receipt with valid, mode, model, and question_count lines",
		Privacy: "validation is local and sends no state to TypeSafe.",
	},
	{
		ID: "ask-native", Purpose: "Send one native request from stdin.", Covers: []string{"ask"},
		Requirements: []string{"installed jeq", "bash", "TYPESAFE_API_KEY"}, Cost: "1 API request",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '{"model":"jev-latest","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}' |
  "$JEQ_BIN" ask --request -`,
		InputShape: "native request JSON on stdin: {model,state,questions}", OutputShape: "one response envelope with _jeq-free answers and usage",
		Privacy: "stdin state is sent to TypeSafe; project response fields before logs.",
	},
	{
		ID: "map-gate", Purpose: "Judge each record and apply an offline threshold.", Covers: []string{"map", "gate"},
		Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "N API requests for N records; jq and gate are offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '{"change":"small"}' '{"change":"large"}' |
  "$JEQ_BIN" map --as risk --input ndjson --state-pointer /change \
    --questions-json '{"questions":{"risk":{"type":"noul","instructions":"Is this low risk?"}}}' |
  "$JEQ_BIN" gate --as policy --input ndjson --value-pointer /_jeq/risk/answers/risk/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.change)'`,
		InputShape: "NDJSON records containing change", OutputShape: "source-free records with _jeq/risk and _jeq/policy",
		Privacy: "map sends each selected record; final jq removes change before logs or sharing.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain.",
	},
	{
		ID: "reduce-gate", Purpose: "Judge one complete collection and gate its aggregate signal.", Covers: []string{"reduce", "gate"},
		Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "1 API request for the complete collection; jq and gate are offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '{"id":"a","value":1}' '{"id":"b","value":2}' |
  "$JEQ_BIN" reduce --as coherent --input ndjson \
    --questions-json '{"questions":{"coherent":{"type":"noul","instructions":"Is this collection coherent?"}}}' |
  "$JEQ_BIN" gate --as policy --value-pointer /_jeq/coherent/answers/coherent/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.items)'`,
		InputShape: "ordered NDJSON records; reduce evaluates the complete array", OutputShape: "source-free aggregate with _jeq/coherent and _jeq/policy",
		Privacy: "the complete collection is sent in one request; final jq removes .items before sharing.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain.",
	},
	{
		ID: "map-reduce-gate", Purpose: "Compose local per-record judgments with one relational aggregate gate.", Covers: []string{"map", "reduce", "gate"},
		Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "N map requests plus 1 reduce request; jq and gate are offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '{"file":{"path":"a","content":"one"}}' '{"file":{"path":"b","content":"two"}}' |
  "$JEQ_BIN" map --as local --input ndjson --state-pointer /file \
    --questions-json '{"questions":{"local":{"type":"noul","instructions":"Is this file focused?"}}}' |
  jq -c '{file:.file,local:._jeq.local.answers.local.noul}' |
  "$JEQ_BIN" reduce --as aggregate --input ndjson \
    --questions-json '{"questions":{"aggregate":{"type":"noul","instructions":"Is this related collection coherent?"}}}' |
  "$JEQ_BIN" gate --as policy --value-pointer /_jeq/aggregate/answers/aggregate/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.items)'`,
		InputShape: "NDJSON records with file:{path,content}", OutputShape: "source-free aggregate/gate evidence after del(.items)",
		Privacy: "source crosses map and reduce; final jq removes .items before logs or sharing.", Exits: "gate exits 0 pass, 10 reject, 11 uncertain; pipefail preserves it.",
	},
}

// NewExamplesCmd creates offline workflow recipe help using native Cobra navigation.
func NewExamplesCmd(_ AskDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples",
		Short: "Discover self-contained workflow recipes",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	for _, recipe := range exampleRecipes {
		recipe := recipe
		cmd.AddCommand(&cobra.Command{
			Use:     recipe.ID,
			Short:   recipe.Purpose,
			Long:    recipeLong(recipe),
			Example: recipe.Shell,
			Args:    cobra.NoArgs,
			RunE:    func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
		})
	}
	return cmd
}

func recipeLong(recipe exampleRecipe) string {
	long := fmt.Sprintf("%s\n\nCommands: %s\nRequirements: %s\nNetwork calls: %s\nInput: %s\nOutput: %s\nPrivacy: %s", recipe.Purpose, strings.Join(recipe.Covers, ", "), strings.Join(recipe.Requirements, ", "), recipe.Cost, recipe.InputShape, recipe.OutputShape, recipe.Privacy)
	if recipe.Exits != "" {
		long += "\nExits: " + recipe.Exits
	}
	return long
}
