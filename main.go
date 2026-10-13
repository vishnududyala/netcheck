// netcheck: a small concurrent TCP port and network health checker.
//
// Usage:
//
//	go run main.go
//	go run main.go -targets="google.com:443,github.com:443,localhost:9999"
//	go run main.go -timeout=500ms
//	go run main.go -json
//
// Concepts used:
//   - goroutines: one per target, so checks run concurrently
//   - channels: collect results safely from the goroutines
//   - sync.WaitGroup: know when all goroutines have finished
//   - net.DialTimeout: the TCP reachability check itself
//   - exit codes: 0 if all targets are UP, 1 if any are DOWN
//   - encoding/json: machine readable output for scripts and CI
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Result holds the outcome of checking a single target.
type Result struct {
	Target  string        `json:"target"`
	Up      bool          `json:"up"`
	Latency time.Duration `json:"latency"`
	Err     string        `json:"error,omitempty"`
}

// jsonResult is a JSON-friendly version of Result, used only for -json output,
// where Latency is converted to plain milliseconds instead of a raw time.Duration.
type jsonResult struct {
	Target    string  `json:"target"`
	Up        bool    `json:"up"`
	LatencyMs float64 `json:"latency_ms"`
	Err       string  `json:"error,omitempty"`
}

// checkTarget attempts a TCP connection to the given host:port and reports
// whether it succeeded, how long it took, and any error encountered.
func checkTarget(target string, timeout time.Duration) Result {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, timeout)
	elapsed := time.Since(start)
	if err != nil {
		return Result{Target: target, Up: false, Latency: elapsed, Err: err.Error()}
	}
	defer conn.Close()
	return Result{Target: target, Up: true, Latency: elapsed}
}

func main() {
	os.Exit(run())
}

func run() int {
	targetsFlag := flag.String(
		"targets",
		"google.com:443,github.com:443,localhost:9999",
		"comma-separated list of host:port targets to check",
	)
	timeoutFlag := flag.Duration("timeout", 2*time.Second, "timeout for each TCP check")
	jsonFlag := flag.Bool("json", false, "print results as JSON instead of a table")

	flag.Parse()

	targets := strings.Split(*targetsFlag, ",")
	filteredTargets := make([]string, 0, len(targets))
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target != "" {
			filteredTargets = append(filteredTargets, target)
		}
	}
	targets = filteredTargets

	results := make(chan Result, len(targets))
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			results <- checkTarget(target, *timeoutFlag)
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]Result, 0, len(targets))
	for r := range results {
		collected = append(collected, r)
	}

	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Target < collected[j].Target
	})

	if *jsonFlag {
		jsonResults := make([]jsonResult, 0, len(collected))
		for _, r := range collected {
			jsonResults = append(jsonResults, jsonResult{
				Target:    r.Target,
				Up:        r.Up,
				LatencyMs: float64(r.Latency) / float64(time.Millisecond),
				Err:       r.Err,
			})
		}
		payload, err := json.MarshalIndent(jsonResults, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to marshal results: %v\n", err)
			return 1
		}
		fmt.Println(string(payload))
	} else {
		fmt.Println("TARGET               STATUS   LATENCY   ERROR")
		fmt.Println("------               ------   -------   -----")
		for _, r := range collected {
			status := "DOWN"
			if r.Up {
				status = "UP"
			}
			errMsg := ""
			if r.Err != "" {
				errMsg = r.Err
			}
			fmt.Printf("%-20s  %-6s  %8s   %s\n", r.Target, status, r.Latency.Round(time.Millisecond), errMsg)
		}
	}

	if hasDownTarget(collected) {
		return 1
	}
	return 0
}

func hasDownTarget(results []Result) bool {
	for _, r := range results {
		if !r.Up {
			return true
		}
	}
	return false
}
