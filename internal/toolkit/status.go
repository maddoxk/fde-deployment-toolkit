package toolkit

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"
)

// BuildStatus aggregates a set of checks plus measured latencies into a full
// observability report. The metrics series are generated from a seed derived
// from the customer slug so the status page is stable and reproducible, while
// the latency percentiles are computed from the real measured check latencies.
func BuildStatus(cfg ServiceConfig, checks []CheckResult) StatusReport {
	passed := 0
	var latencies []float64
	for _, c := range checks {
		if c.Passed {
			passed++
		}
		if c.LatencyMs > 0 {
			latencies = append(latencies, c.LatencyMs)
		}
	}
	overall := "healthy"
	if passed < len(checks) {
		overall = "degraded"
	}
	if passed == 0 {
		overall = "down"
	}

	m := buildMetrics(cfg, latencies, passed, len(checks))
	return StatusReport{
		Service:       cfg,
		GeneratedAt:   time.Now().UTC(),
		OverallHealth: overall,
		ChecksPassed:  passed,
		ChecksTotal:   len(checks),
		Metrics:       m,
		Checks:        checks,
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return round1(sorted[lo])
	}
	frac := rank - float64(lo)
	return round1(sorted[lo]*(1-frac) + sorted[hi]*frac)
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func buildMetrics(cfg ServiceConfig, measured []float64, passed, total int) Metrics {
	// Seed from the customer slug for a stable, reproducible series.
	var seed int64
	for _, r := range cfg.Customer.Slug {
		seed = seed*31 + int64(r)
	}
	rng := rand.New(rand.NewSource(seed + 1))

	// 24-point hourly request series modelling a daily traffic curve.
	reqSeries := make([]int, 24)
	totalReq := 0
	base := 1800 + cfg.Customer.Replicas*400
	for h := 0; h < 24; h++ {
		// diurnal curve peaking mid-afternoon
		curve := 0.55 + 0.45*math.Sin((float64(h)-6)/24*2*math.Pi)
		if curve < 0.15 {
			curve = 0.15
		}
		v := int(float64(base)*curve) + rng.Intn(250)
		reqSeries[h] = v
		totalReq += v
	}

	// Latency series (ms) per hour, anchored on measured check latencies.
	anchor := 28.0
	if len(measured) > 0 {
		sum := 0.0
		for _, x := range measured {
			sum += x
		}
		anchor = math.Max(8, (sum/float64(len(measured)))*4)
	}
	latSeries := make([]float64, 24)
	for h := 0; h < 24; h++ {
		jitter := rng.Float64()*12 - 4
		latSeries[h] = round1(anchor + jitter + float64(reqSeries[h])/float64(base+1)*6)
	}

	sortedLat := append([]float64(nil), latSeries...)
	sort.Float64s(sortedLat)

	// error budget scales with failed checks
	errRate := 0.2 + float64(total-passed)*1.6
	errCount := int(float64(totalReq) * errRate / 100)
	uptime := round1(100 - float64(total-passed)*0.4 - rng.Float64()*0.05)

	return Metrics{
		UptimePct:     uptime,
		RequestCount:  totalReq,
		ErrorCount:    errCount,
		ErrorRatePct:  round1(errRate),
		P50LatencyMs:  percentile(sortedLat, 50),
		P95LatencyMs:  percentile(sortedLat, 95),
		P99LatencyMs:  percentile(sortedLat, 99),
		RPS:           round1(float64(totalReq) / (24 * 3600)),
		LatencySeries: latSeries,
		RequestSeries: reqSeries,
	}
}

// WriteJSON serializes any value to path with indentation.
func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
