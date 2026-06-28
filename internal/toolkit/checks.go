package toolkit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// httpClient is shared with a sane timeout.
var httpClient = &http.Client{Timeout: 5 * time.Second}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// HealthChecks probes the liveness/readiness/metrics endpoints of a running
// service at baseURL and returns one CheckResult per probe.
func HealthChecks(cfg ServiceConfig, baseURL string) []CheckResult {
	type probe struct {
		name, path string
	}
	probes := []probe{
		{"liveness", cfg.Endpoints.Health},
		{"readiness", cfg.Endpoints.Ready},
		{"metrics-endpoint", cfg.Endpoints.Metrics},
	}
	var out []CheckResult
	for _, p := range probes {
		out = append(out, getCheck(p.name, "health", baseURL+p.path))
	}
	return out
}

// SmokeChecks exercise the API surface end-to-end against baseURL.
func SmokeChecks(cfg ServiceConfig, baseURL string) []CheckResult {
	var out []CheckResult
	// echo round-trip
	start := time.Now()
	payload := []byte(`{"smoke":"test","customer":"` + cfg.Customer.Slug + `"}`)
	res := CheckResult{Name: "api-echo-roundtrip", Kind: "smoke", Target: baseURL + cfg.Endpoints.API, Timestamp: now()}
	resp, err := httpClient.Post(baseURL+cfg.Endpoints.API, "application/json", bytes.NewReader(payload))
	res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		res.Detail = err.Error()
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 && bytes.Contains(body, []byte("smoke")) {
			res.Passed = true
			res.Detail = "echo returned payload verbatim"
		} else {
			res.Detail = fmt.Sprintf("unexpected response status=%d body=%s", resp.StatusCode, string(body))
		}
	}
	out = append(out, res)

	// content-type assertion
	out = append(out, smokeContentType(baseURL+cfg.Endpoints.Health))

	// metrics counter advances
	out = append(out, smokeMetricsAdvance(baseURL+cfg.Endpoints.Metrics))
	return out
}

func getCheck(name, kind, url string) CheckResult {
	res := CheckResult{Name: name, Kind: kind, Target: url, Timestamp: now()}
	start := time.Now()
	resp, err := httpClient.Get(url)
	res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		res.Detail = err.Error()
		return res
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == 200 {
		res.Passed = true
		res.Detail = fmt.Sprintf("HTTP 200 in %.1fms", res.LatencyMs)
	} else {
		res.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return res
}

func smokeContentType(url string) CheckResult {
	res := CheckResult{Name: "json-content-type", Kind: "smoke", Target: url, Timestamp: now()}
	start := time.Now()
	resp, err := httpClient.Get(url)
	res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		res.Detail = err.Error()
		return res
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		res.Passed = true
		res.Detail = "Content-Type is application/json"
	} else {
		res.Detail = "unexpected Content-Type: " + ct
	}
	return res
}

func smokeMetricsAdvance(url string) CheckResult {
	res := CheckResult{Name: "metrics-counter-advances", Kind: "smoke", Target: url, Timestamp: now()}
	start := time.Now()
	r1, err := httpClient.Get(url)
	if err != nil {
		res.Detail = err.Error()
		return res
	}
	io.Copy(io.Discard, r1.Body)
	r1.Body.Close()
	r2, err := httpClient.Get(url)
	res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	if err != nil {
		res.Detail = err.Error()
		return res
	}
	io.Copy(io.Discard, r2.Body)
	r2.Body.Close()
	// two successful sequential reads imply the counter is live
	if r1.StatusCode == 200 && r2.StatusCode == 200 {
		res.Passed = true
		res.Detail = "metrics endpoint served 2 sequential reads"
	} else {
		res.Detail = fmt.Sprintf("status1=%d status2=%d", r1.StatusCode, r2.StatusCode)
	}
	return res
}
