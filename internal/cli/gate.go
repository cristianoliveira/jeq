package cli

import (
	"fmt"
	"math"

	"github.com/cristianoliveira/gev/internal/domain/gev"
	"github.com/cristianoliveira/gev/internal/domain/pipeline"
	"github.com/spf13/cobra"
)

// NewGateCmd creates the offline numeric policy command.
func NewGateCmd(deps AskDeps) *cobra.Command {
	var name, input, pointer string
	var passMin, rejectMax float64
	cmd := &cobra.Command{
		Use:   "gate",
		Short: "Apply an offline probability policy to each JSON record",
		Example: `  gev gate --as policy --value-pointer /_gev/risk/answers/risk/noul --pass-min 0.80 --reject-max 0.40
  gev examples map-gate`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runGate(cmd, deps, gateFlags{
				name: name, input: input, pointer: pointer, passMin: passMin, rejectMax: rejectMax,
				nameSet: cmd.Flags().Changed("as"), inputSet: cmd.Flags().Changed("input"),
				pointerSet: cmd.Flags().Changed("value-pointer"), passSet: cmd.Flags().Changed("pass-min"),
				rejectSet: cmd.Flags().Changed("reject-max"),
			})
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&name, "as", "", "evidence name (required)")
	flags.StringVar(&input, "input", "json", "input framing: json or ndjson")
	flags.StringVar(&pointer, "value-pointer", "", "RFC 6901 pointer to numeric value")
	flags.Float64Var(&passMin, "pass-min", 0, "inclusive pass threshold")
	flags.Float64Var(&rejectMax, "reject-max", 0, "inclusive reject threshold")
	return cmd
}

type gateFlags struct {
	name, input, pointer                              string
	passMin, rejectMax                                float64
	nameSet, inputSet, pointerSet, passSet, rejectSet bool
}

type policyStatus struct{ status int }

func (e *policyStatus) Error() string { return fmt.Sprintf("gate policy status %d", e.status) }

func runGate(cmd *cobra.Command, deps AskDeps, f gateFlags) error {
	if output, err := commandOutput(cmd); err != nil || output != "json" {
		return unsupportedOutput(output)
	}
	if !f.nameSet || f.name == "" {
		return gev.NewError(gev.CodeInputInvalid, "--as is required")
	}
	if err := pipeline.ValidateName(f.name); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if err := pipeline.ValidatePointer(f.pointer); err != nil {
		return gev.NewError(gev.CodeInputInvalid, err.Error())
	}
	if !f.pointerSet {
		return gev.NewError(gev.CodeInputInvalid, "--value-pointer is required")
	}
	if !f.passSet || !f.rejectSet {
		return gev.NewError(gev.CodeInputInvalid, "--pass-min and --reject-max are required")
	}
	if f.input != "json" && f.input != "ndjson" {
		return gev.NewError(gev.CodeInputInvalid, "--input must be json or ndjson")
	}
	if math.IsNaN(f.passMin) || math.IsNaN(f.rejectMax) || math.IsInf(f.passMin, 0) || math.IsInf(f.rejectMax, 0) || f.rejectMax < 0 || f.passMin > 1 || f.rejectMax >= f.passMin {
		return gev.NewError(gev.CodeInputInvalid, "thresholds must satisfy 0 <= reject-max < pass-min <= 1")
	}
	if deps.Stdin == nil || deps.ReadStdin == nil {
		return gev.NewError(gev.CodeInputInvalid, "gate stdin is unavailable")
	}
	input, readErr := deps.ReadStdin(deps.Stdin, MapMaxInputBytes, true)
	if readErr != nil {
		return readErr
	}
	records, framingErr := mapRecords(input, f.input)
	if framingErr != nil {
		return framingErr
	}
	counts := map[pipeline.GateDecision]bool{}
	for _, record := range records {
		output, decision, err := pipeline.Gate(record, f.name, f.pointer, pipeline.GatePolicy{PassMin: f.passMin, RejectMax: f.rejectMax})
		if err != nil {
			if f.input == "ndjson" {
				if renderErr := deps.Renderer.RenderError(cmd.OutOrStdout(), err.WithRecovery("fix this record and retry the stream")); renderErr != nil {
					return gev.WrapError(gev.CodeResponseInvalid, renderErr, "writing gate error")
				}
				return &renderedError{err: err}
			}
			return err
		}
		counts[decision] = true
		if err := renderRaw(deps.Renderer, cmd.OutOrStdout(), output); err != nil {
			return gev.WrapError(gev.CodeResponseInvalid, err, "writing gate output")
		}
	}
	if counts[pipeline.DecisionReject] {
		return &policyStatus{status: 10}
	}
	if counts[pipeline.DecisionUncertain] {
		return &policyStatus{status: 11}
	}
	return nil
}
