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
		ID: "noul", Purpose: "Ask a yes/no question and optionally describe what true and false mean.", Covers: []string{"validate"}, Requirements: []string{"installed jeq", "bash"}, Cost: "0 API requests",
		Shell: `printf '%s\n' '{"model":"jev-latest","state":{"file":"report.pdf"},"questions":{"urgent":{"type":"noul","instructions":"Is this file urgent?","criteria":{"true":"Needs immediate attention","false":"Can wait"}}}}' |
  jeq validate --request -`,
		InputShape: "native request JSON with a questions object", OutputShape: "offline validation receipt; Noul answer is a yes/no probability", Privacy: "validation is local and sends no state to TypeSafe.",
	},
	{
		ID: "choice", Purpose: "Categorize a file with a typed Choice question and validate the native request offline.", Covers: []string{"map", "validate"}, Requirements: []string{"installed jeq", "bash", "TYPESAFE_API_KEY for map"}, Cost: "map: 1 API request; validate: 0 API requests",
		Shell: `printf '%s\n' '{"file":"report.pdf"}' |
  jeq map --as category --input ndjson --state-pointer /file \
    --questions-json '{"questions":{"category":{"type":"choice","instructions":"Which category fits this file?","criteria":{"invoice":"Financial document","report":"Analysis or findings"}}}}' &&
printf '%s\n' '{"model":"jev-latest","state":{"file":"report.pdf"},"questions":{"category":{"type":"choice","instructions":"Which category fits this file?","criteria":{"invoice":"Financial document","report":"Analysis or findings"}}}}' |
  jeq validate --request -`,
		InputShape: "JSON records with a file path; native validation request uses {model,state,questions}", OutputShape: "map adds typed Choice evidence under _jeq/category; validate prints an offline receipt",
		Privacy: "map sends selected file state to TypeSafe; validation is local and sends no state.",
	},
	{
		ID: "score", Purpose: "Rate state against ordered levels with a Score question.", Covers: []string{"validate"}, Requirements: []string{"installed jeq", "bash"}, Cost: "0 API requests",
		Shell: `printf '%s\n' '{"model":"jev-latest","state":{"issue":"service outage"},"questions":{"severity":{"type":"score","instructions":"How severe is this issue?","criteria":["Cosmetic: no functional impact","Degraded: important workflow impaired","Critical: service or data at risk"]}}}' |
  jeq validate --request -`,
		InputShape: "native request JSON with a questions object", OutputShape: "offline validation receipt; Score answer rates against ordered levels", Privacy: "validation is local and sends no state to TypeSafe.",
	},
	{
		ID: "rate-sort", Purpose: "Rate records with one Score rubric, then select the highest-scoring record with jq.", Covers: []string{"rate", "jq"}, Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "1 API request per record; jq is offline",
		Shell: `# Run in Bash; rate sends synthetic records to TypeSafe.
set -o pipefail
printf '%s\n' '{"id":"a","description":"minor issue"}' '{"id":"b","description":"service outage"}' |
jeq rate --as severity --input ndjson --state-pointer /description --instruction 'How severe is this issue?' \
  --level 'Cosmetic: no functional impact' --level 'Degraded: an important workflow is impaired' --level 'Critical: service or data is at risk' |
jq -s 'sort_by(._jeq.severity.answers.severity.score) | reverse | .[:1] | map(.id)'`,
		InputShape: "NDJSON records with descriptions", OutputShape: "the highest-scoring record selected explicitly with jq", Privacy: "descriptions are sent independently; jq controls final output", Exits: "rate exits 0 on success; jq is offline.",
	},
	{
		ID: "rank-top-k", Purpose: "Rank candidates once, then select an explicit top-k policy with jq.", Covers: []string{"rank", "jq"},
		Requirements: []string{"installed jeq", "bash", "jq", "TYPESAFE_API_KEY"}, Cost: "1 API request; jq is offline",
		Shell: `# Run in Bash; rank sends synthetic candidates and state to TypeSafe.
set -o pipefail
printf '%s\n' '[{"name":"billing","description":"Payments and refunds"},{"name":"support","description":"Account help"},{"name":"fallback","description":"No specialist match"}]' |
  jeq rank --as route --input json --state 'A customer asks about a refund' \
    --instruction 'Which handler best fits this request?' --id-pointer /name --criteria-pointer /description |
  jq -c '.items[:2] | map(.candidate)'`,
		InputShape: "JSON array of candidate objects", OutputShape: "top-two candidate objects selected explicitly with jq", Privacy: "candidate descriptions and state are sent to TypeSafe; jq emits only selected candidates.", Exits: "rank exits 0 on success; jq is offline.",
	},
	{
		ID: "validate-native", Purpose: "Validate one native request locally before spending a network call.", Covers: []string{"validate"},
		Requirements: []string{"installed jeq", "bash"}, Cost: "0 API requests; validation is offline",
		Shell: `printf '%s\n' '{"model":"jev-latest","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}' |
  jeq validate --request -`,
		InputShape: "native request JSON on stdin: {model,state,questions}", OutputShape: "plain validation receipt with valid, mode, model, and question_count lines",
		Privacy: "validation is local and sends no state to TypeSafe.",
	},
	{
		ID: "ask-native", Purpose: "Send one native request from stdin.", Covers: []string{"ask"},
		Requirements: []string{"installed jeq", "bash", "TYPESAFE_API_KEY"}, Cost: "1 API request",
		Shell: `printf '%s\n' '{"model":"jev-latest","state":{"message":"hello"},"questions":{"urgent":{"type":"noul","instructions":"Is this urgent?"}}}' |
  jeq ask --request -`,
		InputShape: "native request JSON on stdin: {model,state,questions}", OutputShape: "one response envelope with _jeq-free answers and usage",
		Privacy: "stdin state is sent to TypeSafe; project response fields before logs.",
	},
	{
		ID: "debug-chain", Purpose: "Capture safe lifecycle traces for a caller-owned map/rate/reduce chain.", Covers: []string{"verbose", "trace-id", "map", "rate", "reduce"}, Requirements: []string{"installed jeq", "bash", "TYPESAFE_API_KEY"}, Cost: "one request per map/rate record plus one reduce request",
		Shell: `# Run in Bash; creates the three caller-owned trace files in this directory.
set -o pipefail
printf '%s\n' '{"description":"incident"}' |
  jeq --verbose map --as triage --input ndjson --state-pointer /description --questions-json '{"questions":{"triage":{"type":"noul","instructions":"Is this urgent?"}}}' 2>map.trace.ndjson |
  jeq --verbose rate --as severity --input ndjson --state-pointer /description --instruction 'How severe?' --level Low --level High 2>rate.trace.ndjson |
  jeq --verbose reduce --as aggregate --input ndjson --questions-json '{"questions":{"aggregate":{"type":"noul","instructions":"Is this coherent?"}}}' 2>reduce.trace.ndjson
# Trace files are caller-owned; jeq never creates or reloads them.
`,
		InputShape: "NDJSON records", OutputShape: "JSON/NDJSON evidence on stdout; trace events on stderr", Privacy: "traces contain metadata only, never payloads or credentials.", Exits: "all stages must succeed; inspect trace files for lifecycle failures.",
	},
	{
		ID: "map-gate", Purpose: "Judge each record and apply an offline threshold.", Covers: []string{"map", "gate"},
		Requirements: []string{"installed jeq", "Bash", "jq", "mktemp", "TYPESAFE_API_KEY"}, Cost: "N API requests for N records; jq and gate are offline",
		Shell: `# Run in Bash. This uses synthetic data; map sends it to TypeSafe.
set -o pipefail

decisions_file=$(mktemp) || exit $?
trap 'rm -f "$decisions_file"' EXIT

pipeline_status=0
if printf '%s\n' '{"id":"a","change":"small"}' '{"id":"b","change":"large"}' |
  jeq map --as risk --input ndjson --state-pointer /change \
    --questions-json '{"questions":{"risk":{"type":"noul","instructions":"Is this low risk?"}}}' |
  jeq gate --as policy --input ndjson --value-pointer /_jeq/risk/answers/risk/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.change)' >"$decisions_file"; then
  pipeline_status=0
else
  pipeline_status=$?
fi

# Keep every decision visible, including reject and uncertain records.
cat "$decisions_file"
output_status=$?
if ((output_status != 0)); then exit "$output_status"; fi

case "$pipeline_status" in
  0) ;;
  10) printf '%s\n' 'policy rejected at least one record' >&2 ;;
  11) printf '%s\n' 'policy is uncertain for at least one record' >&2 ;;
  *) exit "$pipeline_status" ;;
esac
exit "$pipeline_status"`,
		InputShape: "two synthetic NDJSON records containing id and change", OutputShape: "all records with _jeq/risk and _jeq/policy; source change is removed",
		Privacy: "the synthetic or caller-provided change is sent to TypeSafe; jq removes it from output, not from the request.", Exits: "0 pass; 10 if any record rejects; 11 if uncertain records and none reject. This Bash block handles policy outcomes explicitly.",
	},
	{
		ID: "reduce-gate", Purpose: "Judge one complete collection and gate its aggregate signal.", Covers: []string{"reduce", "gate"},
		Requirements: []string{"installed jeq", "Bash", "jq", "mktemp", "TYPESAFE_API_KEY"}, Cost: "1 API request for the complete collection; jq and gate are offline",
		Shell: `# Run in Bash. This collection is synthetic; reduce sends it to TypeSafe.
set -o pipefail

decisions_file=$(mktemp) || exit $?
trap 'rm -f "$decisions_file"' EXIT

pipeline_status=0
if printf '%s\n' '{"id":"a","value":1}' '{"id":"b","value":2}' |
  jeq reduce --as coherent --input ndjson \
    --questions-json '{"questions":{"coherent":{"type":"noul","instructions":"Is this collection coherent?"}}}' |
  jeq gate --as policy --value-pointer /_jeq/coherent/answers/coherent/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.items)' >"$decisions_file"; then
  pipeline_status=0
else
  pipeline_status=$?
fi

cat "$decisions_file"
output_status=$?
if ((output_status != 0)); then exit "$output_status"; fi
case "$pipeline_status" in
  0) ;;
  10) printf '%s\n' 'policy rejected the collection' >&2 ;;
  11) printf '%s\n' 'policy is uncertain about the collection' >&2 ;;
  *) exit "$pipeline_status" ;;
esac
exit "$pipeline_status"`,
		InputShape: "ordered synthetic NDJSON records; reduce evaluates the complete array", OutputShape: "aggregate with _jeq/coherent and _jeq/policy; source items are removed",
		Privacy: "the full collection is sent in one request; jq removes .items from output, not from the request.", Exits: "0 pass; 10 reject; 11 uncertain. The Bash block handles each gate result explicitly.",
	},
	{
		ID: "map-reduce-gate", Purpose: "Compose local per-record judgments with one relational aggregate gate.", Covers: []string{"map", "reduce", "gate"},
		Requirements: []string{"installed jeq", "Bash", "jq", "mktemp", "TYPESAFE_API_KEY"}, Cost: "N map requests plus 1 reduce request; jq and gate are offline",
		Shell: `# Run in Bash. These files are synthetic; map and reduce send them to TypeSafe.
set -o pipefail

decisions_file=$(mktemp) || exit $?
trap 'rm -f "$decisions_file"' EXIT

pipeline_status=0
if printf '%s\n' '{"file":{"path":"a","content":"one"}}' '{"file":{"path":"b","content":"two"}}' |
  jeq map --as local --input ndjson --state-pointer /file \
    --questions-json '{"questions":{"local":{"type":"noul","instructions":"Is this file focused?"}}}' |
  jq -c '{file:.file,local:._jeq.local.answers.local.noul}' |
  jeq reduce --as aggregate --input ndjson \
    --questions-json '{"questions":{"aggregate":{"type":"noul","instructions":"Is this related collection coherent?"}}}' |
  jeq gate --as policy --value-pointer /_jeq/aggregate/answers/aggregate/noul \
    --pass-min 0.80 --reject-max 0.40 |
  jq -c 'del(.items)' >"$decisions_file"; then
  pipeline_status=0
else
  pipeline_status=$?
fi

cat "$decisions_file"
output_status=$?
if ((output_status != 0)); then exit "$output_status"; fi
case "$pipeline_status" in
  0) ;;
  10) printf '%s\n' 'policy rejected the collection' >&2 ;;
  11) printf '%s\n' 'policy is uncertain about the collection' >&2 ;;
  *) exit "$pipeline_status" ;;
esac
exit "$pipeline_status"`,
		InputShape: "synthetic NDJSON records with file:{path,content}", OutputShape: "aggregate with _jeq/aggregate and _jeq/policy; source items are removed",
		Privacy: "source crosses map and reduce; jq removes .items from output, not from either request.", Exits: "0 pass; 10 reject; 11 uncertain. The Bash block handles each gate result explicitly.",
	},
}

// NewExamplesCmd creates offline workflow recipe help using native Cobra navigation.
func NewExamplesCmd(_ AskDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "examples",
		Short: "Discover self-contained workflow recipes (Jev evidence, jq/shell policy)",
		Long:  "Look up examples by workflow name or jeq command name (for example, jeq examples map). Jev maps natural-language state to caller-defined typed decisions and probabilities; jq and the shell own deterministic policy and actions. Jev does not write replies, produce code, or return reasoning explanations.",
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
	covered := map[string][]exampleRecipe{}
	for _, recipe := range exampleRecipes {
		for _, command := range recipe.Covers {
			covered[command] = append(covered[command], recipe)
		}
	}
	canonical := map[string]string{"map": "map-gate", "reduce": "reduce-gate"}
	for _, command := range []string{"ask", "map", "rate", "rank", "reduce", "gate", "validate"} {
		recipes := covered[command]
		if id := canonical[command]; id != "" {
			for i, recipe := range recipes {
				if recipe.ID == id {
					recipes[0], recipes[i] = recipes[i], recipes[0]
					break
				}
			}
		}
		if len(recipes) == 0 {
			continue
		}
		cmdName := command
		primary := recipes[0]
		related := make([]string, 0, len(recipes)-1)
		for _, recipe := range recipes[1:] {
			related = append(related, recipe.ID)
		}
		long := fmt.Sprintf("Canonical offline recipe for jeq %s: %s", command, primary.ID)
		if len(related) > 0 {
			long += "\nRelated recipes: " + strings.Join(related, ", ")
		}
		cmd.AddCommand(&cobra.Command{Use: cmdName, Short: "Find the canonical " + cmdName + " example", Long: long, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }, Example: "jeq examples " + primary.ID})
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
