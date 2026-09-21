package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	records := flag.Int("records", 100, "number of NDJSON records")
	flag.Parse()
	if *records < 1 {
		fmt.Fprintln(os.Stderr, "records must be positive")
		os.Exit(2)
	}
	var requests, active, peak atomic.Int64
	gate := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.HasPrefix(string(body), "{") {
			http.Error(w, "bad request", 400)
			return
		}
		requests.Add(1)
		current := active.Add(1)
		for {
			old := peak.Load()
			if current <= old || peak.CompareAndSwap(old, current) {
				break
			}
		}
		once.Do(func() { close(gate) })
		<-gate
		active.Add(-1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"model":"local","answers":{"risk":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	defer server.Close()
	binary, err := exec.LookPath("jeq")
	if err != nil {
		fmt.Fprintln(os.Stderr, "jeq not found in PATH")
		os.Exit(2)
	}
	var input bytes.Buffer
	for i := 0; i < *records; i++ {
		fmt.Fprintf(&input, `{"state":"record-%d"}`+"\n", i)
	}
	cmd := exec.Command(binary, "map", "--input", "ndjson", "--as", "risk", "--state-pointer", "/state", "--questions-json", `{"questions":{"risk":{"type":"noul","instructions":"is it safe?"}}}`)
	cmd.Stdin = &input
	cmd.Env = append(os.Environ(), "TYPESAFE_BASE_URL="+server.URL, "TYPESAFE_API_KEY=local-only", "JEQ_MODEL=local")
	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %s", err, out)
		os.Exit(1)
	}
	scanner := bufio.NewScanner(bytes.NewReader(out))
	count := 0
	ordered := true
	for scanner.Scan() {
		var doc map[string]any
		if json.Unmarshal(scanner.Bytes(), &doc) != nil {
			ordered = false
			break
		}
		state, ok := doc["state"].(string)
		if !ok || state != fmt.Sprintf("record-%d", count) {
			ordered = false
		}
		count++
	}
	peakValue := peak.Load()
	pass := count == *records && ordered && requests.Load() == int64(*records) && peakValue > 1 && peakValue <= 4
	fmt.Printf("records=%d requests=%d peak=%d elapsed=%s order=%t %s\n", *records, requests.Load(), peakValue, elapsed.Round(time.Millisecond), ordered, map[bool]string{true: "PASS", false: "FAIL"}[pass])
	if !pass {
		os.Exit(1)
	}
}
