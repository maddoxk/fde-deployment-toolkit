package toolkit

import "time"

// Customer holds onboarding parameters supplied on the CLI.
type Customer struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Environment string `json:"environment"`
	Region      string `json:"region"`
	Tier        string `json:"tier"`
	Port        int    `json:"port"`
	Replicas    int    `json:"replicas"`
}

// ServiceConfig is the rendered configuration for the scaffolded sample service.
type ServiceConfig struct {
	Customer    Customer  `json:"customer"`
	ServiceName string    `json:"serviceName"`
	Image       string    `json:"image"`
	GeneratedAt time.Time `json:"generatedAt"`
	Endpoints   Endpoints `json:"endpoints"`
}

// Endpoints describes the service's exposed routes.
type Endpoints struct {
	Health  string `json:"health"`
	Ready   string `json:"ready"`
	Metrics string `json:"metrics"`
	API     string `json:"api"`
}

// CheckResult is a single health or smoke check outcome.
type CheckResult struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Target    string  `json:"target"`
	Passed    bool    `json:"passed"`
	LatencyMs float64 `json:"latencyMs"`
	Detail    string  `json:"detail"`
	Timestamp string  `json:"timestamp"`
}

// Metrics is the observability snapshot rendered onto the status page.
type Metrics struct {
	UptimePct     float64   `json:"uptimePct"`
	RequestCount  int       `json:"requestCount"`
	ErrorCount    int       `json:"errorCount"`
	ErrorRatePct  float64   `json:"errorRatePct"`
	P50LatencyMs  float64   `json:"p50LatencyMs"`
	P95LatencyMs  float64   `json:"p95LatencyMs"`
	P99LatencyMs  float64   `json:"p99LatencyMs"`
	RPS           float64   `json:"rps"`
	LatencySeries []float64 `json:"latencySeries"`
	RequestSeries []int     `json:"requestSeries"`
}

// StatusReport is the full observability payload serialized to status.json
// and rendered into the static status page.
type StatusReport struct {
	Service       ServiceConfig `json:"service"`
	GeneratedAt   time.Time     `json:"generatedAt"`
	OverallHealth string        `json:"overallHealth"`
	ChecksPassed  int           `json:"checksPassed"`
	ChecksTotal   int           `json:"checksTotal"`
	Metrics       Metrics       `json:"metrics"`
	Checks        []CheckResult `json:"checks"`
}
