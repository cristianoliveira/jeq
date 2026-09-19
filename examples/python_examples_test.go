package examples_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runPython(t *testing.T, script, input, endpoint string, extra map[string]string) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", filepath.Join(repoRoot, script))
	cmd.Dir = repoRoot
	cmd.Env = envWith(map[string]string{
		"GEV_BIN":          gevBin,
		"GEV_BASE_URL":     endpoint,
		"GEV_MODEL":        "jev-latest",
		"TYPESAFE_API_KEY": "examples-test-key",
		"NO_COLOR":         "1",
		"TERM":             "dumb",
	}, extra)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		t.Fatalf("%s timed out: stdout=%q stderr=%q", script, stdout.String(), stderr.String())
	}
	return processResult{stdout: stdout.String(), stderr: stderr.String(), exit: processExit(cmd)}
}

func choiceAnswer(choice string, confidence float64, candidates ...string) map[string]any {
	probabilities := make(map[string]float64, len(candidates))
	for _, candidate := range candidates {
		probabilities[candidate] = 0.1
	}
	probabilities[choice] = 0.8
	return map[string]any{
		"type": "choice", "choice": choice, "confidence": confidence,
		"probabilities": probabilities,
	}
}

func pythonScore(value, confidence float64) map[string]any {
	return answerScoreWithConfidence(value, confidence)
}

func assertNoRawInput(t *testing.T, output, raw string) {
	t.Helper()
	if strings.Contains(output, raw) {
		t.Fatalf("receipt leaked raw input %q: %s", raw, output)
	}
}

func assertPythonRequest(t *testing.T, body map[string]any, state any, names ...string) {
	t.Helper()
	assertQuestions(t, body, names...)
	if body["state"] == nil {
		t.Fatalf("request omitted state: %#v", body)
	}
	got, _ := json.Marshal(body["state"])
	want, _ := json.Marshal(state)
	if string(got) != string(want) {
		t.Fatalf("state=%s want=%s", got, want)
	}
}

func TestPythonSupportRouterReceiptAndConfidencePolicy(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		conf   float64
		action string
	}{
		{name: "allowlisted", conf: 0.9, action: "technical_queue"},
		{name: "low confidence", conf: 0.4, action: "human_review"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ticket := "Python router ticket " + tc.name
			api := newFakeAPI(t, func(int) (int, []byte) {
				return http.StatusOK, responseDocument(map[string]any{
					"route":    choiceAnswer("technical", tc.conf, "billing", "technical", "sales"),
					"urgent":   answerNoul(0.8),
					"escalate": answerNoul(0.2),
				})
			})
			result := runPython(t, "examples/python/support_router.py", ticket, api.server.URL, nil)
			if result.exit != 0 || result.stderr != "" {
				t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
			}
			receipt := oneJSON(t, result.stdout)
			decision := receipt["decision"].(map[string]any)
			if decision["action"] != tc.action || receipt["stage_count"] != float64(1) {
				t.Fatalf("receipt=%#v", receipt)
			}
			assertUsage(t, receipt)
			assertNoRawInput(t, result.stdout, ticket)
			assertPythonRequest(t, api.body(t, 0), ticket, "route", "urgent", "escalate")
		})
	}
}

func TestPythonReleaseReadinessBlockPolicyAndUncertainty(t *testing.T) {
	t.Parallel()
	t.Run("local blocker makes zero calls", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) { return http.StatusOK, responseDocument(nil) })
		input := `{"change_id":"blocked-release","tests_passed":false,"known_vulnerabilities":[]}`
		result := runPython(t, "examples/python/release_readiness.py", input, api.server.URL, nil)
		if result.exit != 10 || api.count() != 0 || result.stderr != "" {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, api.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"] != "block" || receipt["stage_count"] != float64(0) {
			t.Fatalf("receipt=%#v", receipt)
		}
		assertNoRawInput(t, result.stdout, "blocked-release")
	})
	t.Run("semantic pass preserves evidence", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"risk":          pythonScore(0.2, 0.9),
				"manual_review": answerNoul(0.1),
				"rollout":       choiceAnswer("full", 0.9, "full", "canary", "hold"),
			})
		})
		input := `{"change_id":"ready-release","tests_passed":true,"known_vulnerabilities":[]}`
		result := runPython(t, "examples/python/release_readiness.py", input, api.server.URL, nil)
		if result.exit != 0 || result.stderr != "" {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"] != "pass" || receipt["stage_count"] != float64(1) {
			t.Fatalf("receipt=%#v", receipt)
		}
		assertUsage(t, receipt)
		trace := receipt["policy_trace"].(map[string]any)
		if trace["risk_raw_score"] != float64(0.2) || trace["risk_normalized"] != float64(0.066667) {
			t.Fatalf("policy trace=%#v", trace)
		}
		assertNoRawInput(t, result.stdout, "ready-release")
		assertPythonRequest(t, api.body(t, 0), map[string]any{"change_id": "ready-release", "tests_passed": true, "known_vulnerabilities": []any{}}, "risk", "manual_review", "rollout")
	})
	t.Run("raw high risk score blocks after normalization", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"risk":          pythonScore(2.4, 0.9),
				"manual_review": answerNoul(0.1),
				"rollout":       choiceAnswer("full", 0.9, "full", "canary", "hold"),
			})
		})
		result := runPython(t, "examples/python/release_readiness.py", `{"tests_passed":true}`, api.server.URL, nil)
		if result.exit != 10 || result.stderr != "" {
			t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"] != "block" {
			t.Fatalf("receipt=%#v", receipt)
		}
		trace := receipt["policy_trace"].(map[string]any)
		if trace["risk_raw_score"] != 2.4 || trace["risk_normalized"] != 0.8 {
			t.Fatalf("policy trace=%#v", trace)
		}
	})
	t.Run("low confidence is uncertain", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"risk":          pythonScore(0.2, 0.4),
				"manual_review": answerNoul(0.1),
				"rollout":       choiceAnswer("full", 0.9, "full", "canary", "hold"),
			})
		})
		result := runPython(t, "examples/python/release_readiness.py", `{"tests_passed":true}`, api.server.URL, nil)
		if result.exit != 11 || api.count() != 1 || result.stderr != "" {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, api.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"] != "uncertain" {
			t.Fatalf("receipt=%#v", receipt)
		}
	})
}

func TestPythonIncidentCascadeAndCandidateValidation(t *testing.T) {
	t.Parallel()
	incident := map[string]any{"incident_id": "INC-PY-1", "details": "API latency"}
	inputBytes, _ := json.Marshal(incident)
	t.Run("two-stage happy path", func(t *testing.T) {
		api := newFakeAPI(t, func(count int) (int, []byte) {
			if count == 1 {
				return http.StatusOK, responseDocument(map[string]any{
					"category": choiceAnswer("technical", 0.9, "billing", "technical", "security"),
					"severity": pythonScore(2.4, 0.9),
					"page":     answerNoul(0.9),
				})
			}
			return http.StatusOK, responseDocument(map[string]any{
				"runbook": choiceAnswer("technical-api-outage", 0.9, "technical-service-degraded", "technical-api-outage"),
			})
		})
		result := runPython(t, "examples/python/incident_triage.py", string(inputBytes), api.server.URL, nil)
		if result.exit != 0 || api.count() != 2 || result.stderr != "" {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, api.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		decision := receipt["decision"].(map[string]any)
		if decision["action"] != "runbook_selected" || receipt["stage_count"] != float64(2) {
			t.Fatalf("receipt=%#v", receipt)
		}
		usage := receipt["usage"].(map[string]any)
		if usage["input_tokens"] != float64(20) || usage["output_tokens"] != float64(4) {
			t.Fatalf("aggregated usage=%#v", receipt["usage"])
		}
		assertNoRawInput(t, result.stdout, "INC-PY-1")
		assertPythonRequest(t, api.body(t, 0), incident, "category", "severity", "page")
		second := api.body(t, 1)
		assertPythonRequest(t, second, incident, "runbook")
		criteria := second["questions"].(map[string]any)["runbook"].(map[string]any)["criteria"].(map[string]any)
		if len(criteria) != 2 || criteria["technical-api-outage"] == nil || criteria["billing-payment-failure"] != nil {
			t.Fatalf("candidate criteria=%#v", criteria)
		}
	})
	t.Run("low confidence stops before second call", func(t *testing.T) {
		api := newFakeAPI(t, func(int) (int, []byte) {
			return http.StatusOK, responseDocument(map[string]any{
				"category": choiceAnswer("technical", 0.4, "billing", "technical", "security"),
				"severity": pythonScore(0.8, 0.9),
				"page":     answerNoul(0.9),
			})
		})
		result := runPython(t, "examples/python/incident_triage.py", string(inputBytes), api.server.URL, nil)
		if result.exit != 11 || api.count() != 1 || result.stderr != "" {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, api.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"].(map[string]any)["action"] != "human_review" || receipt["stage_count"] != float64(1) {
			t.Fatalf("receipt=%#v", receipt)
		}
	})
	t.Run("invalid selected runbook is rejected", func(t *testing.T) {
		api := newFakeAPI(t, func(count int) (int, []byte) {
			if count == 1 {
				return http.StatusOK, responseDocument(map[string]any{
					"category": choiceAnswer("technical", 0.9, "billing", "technical", "security"),
					"severity": pythonScore(0.8, 0.9),
					"page":     answerNoul(0.9),
				})
			}
			return http.StatusOK, responseDocument(map[string]any{
				"runbook": choiceAnswer("billing-payment-failure", 0.9, "billing-payment-failure"),
			})
		})
		result := runPython(t, "examples/python/incident_triage.py", string(inputBytes), api.server.URL, nil)
		if result.exit != 11 || api.count() != 2 || result.stderr != "" {
			t.Fatalf("exit=%d requests=%d stderr=%q stdout=%q", result.exit, api.count(), result.stderr, result.stdout)
		}
		receipt := oneJSON(t, result.stdout)
		if receipt["decision"].(map[string]any)["action"] != "human_review" || receipt["stage_count"] != float64(2) {
			t.Fatalf("receipt=%#v", receipt)
		}
		usage := receipt["usage"].(map[string]any)
		if usage["input_tokens"] != float64(20) || usage["output_tokens"] != float64(4) {
			t.Fatalf("review usage=%#v", receipt["usage"])
		}
	})
}

func TestPythonOperationalStatusPropagation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{name: "auth status one", status: 1, body: ""},
		{name: "usage status two", status: 2, body: ""},
		{name: "interrupt status 130", status: 130, body: `{"code":"GEV_INTERRUPTED","message":"interrupted"}
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.status == 1 {
				api := newFakeAPI(t, func(int) (int, []byte) { return http.StatusUnauthorized, []byte(`{"message":"denied"}`) })
				result := runPython(t, "examples/python/release_readiness.py", `{"tests_passed":true}`, api.server.URL, nil)
				if result.exit != tc.status || result.stderr != "" {
					t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
				}
				doc := oneJSON(t, result.stdout)
				if doc["code"] != "GEV_AUTH_REJECTED" {
					t.Fatalf("error=%#v", doc)
				}
				return
			}
			if tc.status == 2 {
				wrapper := writeWrapper(t, `exec "$GEV_REAL" "$@" --output toon`)
				result := runPython(t, "examples/python/release_readiness.py", `{"tests_passed":true}`, "", map[string]string{"GEV_BIN": wrapper, "GEV_REAL": gevBin})
				if result.exit != tc.status || result.stderr != "" {
					t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
				}
				doc := oneJSON(t, result.stdout)
				if doc["code"] != "GEV_INPUT_INVALID" {
					t.Fatalf("error=%#v", doc)
				}
				return
			}
			wrapper := writeWrapper(t, "printf '%s' '"+tc.body+"'; exit 130")
			result := runPython(t, "examples/python/release_readiness.py", `{"tests_passed":true}`, "", map[string]string{"GEV_BIN": wrapper})
			if result.exit != tc.status || result.stderr != "" {
				t.Fatalf("exit=%d stderr=%q stdout=%q", result.exit, result.stderr, result.stdout)
			}
			if oneJSON(t, result.stdout)["code"] != "GEV_INTERRUPTED" {
				t.Fatalf("error=%q", result.stdout)
			}
		})
	}
}
