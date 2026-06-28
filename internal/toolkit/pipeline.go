package toolkit

import (
	"context"
	"time"
)

// RunAgainstLiveService starts the sample service, runs the requested check
// kinds against it, builds a status report and stops the service. It is used by
// `healthcheck`, `smoke` and `status` so checks always run against a real,
// freshly-started service rather than fabricated data.
//
// kinds may contain "health" and/or "smoke".
func RunAgainstLiveService(cfg ServiceConfig, kinds []string) (StatusReport, error) {
	svc, err := NewSampleService(cfg)
	if err != nil {
		return StatusReport{}, err
	}
	svc.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = svc.Stop(ctx)
	}()

	// brief settle so the listener is accepting
	time.Sleep(40 * time.Millisecond)
	base := svc.BaseURL()

	var checks []CheckResult
	for _, k := range kinds {
		switch k {
		case "health":
			checks = append(checks, HealthChecks(cfg, base)...)
		case "smoke":
			checks = append(checks, SmokeChecks(cfg, base)...)
		}
	}
	return BuildStatus(cfg, checks), nil
}
