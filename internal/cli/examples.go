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
		ID: "rate-sort", Purpose: "Rate every record with one Score rubric, then sort explicitly with jq.", Covers: []string{"rate", "jq"}, Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "1 API request per record; jq is offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
rated=$(mktemp); trap 'rm -f "$rated"' EXIT
printf '%s\n' '{"id":"a","description":"minor issue"}' '{"id":"b","description":"service outage"}' |
  "$JEQ_BIN" rate --as severity --input ndjson --state-pointer /description --instruction 'How severe is this issue?' \
    --level 'Cosmetic: no functional impact' --level 'Degraded: an important workflow is impaired' --level 'Critical: service or data is at risk' > "$rated"
# Sort, select top-k, filter by a caller-owned threshold, and apply explicit policy offline.
jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse' "$rated"
jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse | .[:1] | map(.id)' "$rated"
jq 'select(._jeq.severity.answers.severity.score >= 1.5)' "$rated"
jq -s 'map(if ._jeq.severity.answers.severity.score >= 1.5 then .id else empty end)' "$rated"`,
		InputShape: "NDJSON records with descriptions", OutputShape: "all records sorted by returned semantic Score", Privacy: "descriptions are sent independently; jq controls final output", Exits: "rate exits 0 on success; jq is offline.",
	},
	{
		ID: "rank-top-k", Purpose: "Rank candidates once, then select an explicit top-k policy with jq.", Covers: []string{"rank", "jq"},
		Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "1 API request; jq is offline",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
printf '%s\n' '[{"name":"billing","description":"Payments and refunds"},{"name":"support","description":"Account help"},{"name":"fallback","description":"No specialist match"}]' |
  "$JEQ_BIN" rank --as route --input json --state 'A customer asks about a refund' \
    --instruction 'Which handler best fits this request?' --id-pointer /name --criteria-pointer /description |
  jq -c '.items[:2] | map(.candidate)'`,
		InputShape: "JSON array of candidate objects", OutputShape: "top-two candidate objects selected explicitly with jq", Privacy: "candidate descriptions and state are sent to TypeSafe; jq emits only selected candidates.", Exits: "rank exits 0 on success; jq is offline.",
	},
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
		ID: "debug-chain", Purpose: "Capture safe lifecycle traces for a caller-owned map/rate/reduce chain.", Covers: []string{"verbose", "trace-id", "map", "rate", "reduce"}, Requirements: []string{"installed jeq", "bash", "TYPESAFE_API_KEY"}, Cost: "one request per map/rate record plus one reduce request",
		Shell: `JEQ_BIN=${JEQ_BIN:-jeq}
set -euo pipefail
export JEQ_TRACE_ID=${JEQ_TRACE_ID:-debug-chain}
printf '%s\n' '{"description":"incident"}' |
  "$JEQ_BIN" --verbose map --as triage --input ndjson --state-pointer /description --questions-json '{"questions":{"triage":{"type":"noul","instructions":"Is this urgent?"}}}' 2>map.trace.ndjson |
  "$JEQ_BIN" --verbose rate --as severity --input ndjson --state-pointer /description --instruction 'How severe?' --level Low --level High 2>rate.trace.ndjson |
  "$JEQ_BIN" --verbose reduce --as aggregate --input ndjson --questions-json '{"questions":{"aggregate":{"type":"noul","instructions":"Is this coherent?"}}}' 2>reduce.trace.ndjson
# Trace files are caller-owned; JEQ never creates or reloads them.
`,
		InputShape: "NDJSON records", OutputShape: "JSON/NDJSON evidence on stdout; trace events on stderr", Privacy: "traces contain metadata only, never payloads or credentials.", Exits: "all stages must succeed; inspect trace files for lifecycle failures.",
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
		Short: "Discover self-contained workflow recipes (Jev evidence, jq/shell policy)",
		Long:  "Jev maps natural-language state to caller-defined typed decisions and probabilities; jq and the shell own deterministic policy and actions. Jev does not write replies, produce code, or return reasoning explanations.",
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
	role := "Jev supplies typed semantic evidence; jq and the shell own projection, deterministic policy, and actions."
	if recipe.ID == "validate-native" {
		role = "This recipe does not call Jev: it validates the caller-defined request shape locally before spend."
	}
	long := fmt.Sprintf("%s\n\n%s\n\nCommands: %s\nRequirements: %s\nNetwork calls: %s\nInput: %s\nOutput: %s\nPrivacy: %s", recipe.Purpose, role, strings.Join(recipe.Covers, ", "), strings.Join(recipe.Requirements, ", "), recipe.Cost, recipe.InputShape, recipe.OutputShape, recipe.Privacy)
	if recipe.Exits != "" {
		long += "\nExits: " + recipe.Exits
	}
	return long
}
