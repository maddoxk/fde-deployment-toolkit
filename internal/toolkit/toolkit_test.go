package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Acme Corp":         "acme-corp",
		"  North Wind LLC ": "north-wind-llc",
		"Foo_Bar/Baz!":      "foo-bar-baz",
		"Already-slug":      "already-slug",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestBuildConfigDefaults(t *testing.T) {
	cfg := BuildConfig(Customer{Name: "Globex Inc"})
	if cfg.Customer.Slug != "globex-inc" {
		t.Errorf("slug=%q", cfg.Customer.Slug)
	}
	if cfg.Customer.Port != 8080 || cfg.Customer.Replicas != 2 {
		t.Errorf("defaults not applied: %+v", cfg.Customer)
	}
	if cfg.ServiceName != "globex-inc-sample-svc" {
		t.Errorf("serviceName=%q", cfg.ServiceName)
	}
	if !strings.HasPrefix(cfg.Endpoints.Health, "/") {
		t.Errorf("bad health endpoint %q", cfg.Endpoints.Health)
	}
}

func TestScaffoldWritesArtifacts(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Scaffold(Customer{Name: "Initech", Tier: "premium"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"service.config.json", "deploy.yaml", "sample_service.go.txt"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing artifact %s: %v", f, err)
		}
	}
	man, _ := os.ReadFile(filepath.Join(dir, "deploy.yaml"))
	if !strings.Contains(string(man), "initech-sample-svc") {
		t.Errorf("manifest missing service name:\n%s", man)
	}
	if !strings.Contains(string(man), cfg.Image) {
		t.Errorf("manifest missing image %s", cfg.Image)
	}
}

func TestSampleServiceServesEndpoints(t *testing.T) {
	cfg := BuildConfig(Customer{Name: "Hooli", Port: 0})
	svc, err := NewSampleService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.Start()
	defer svc.Stop(testCtx(t))
	base := svc.BaseURL()

	checks := HealthChecks(cfg, base)
	if len(checks) != 3 {
		t.Fatalf("expected 3 health checks, got %d", len(checks))
	}
	for _, c := range checks {
		if !c.Passed {
			t.Errorf("health check %s failed: %s", c.Name, c.Detail)
		}
	}
	smokes := SmokeChecks(cfg, base)
	if len(smokes) != 3 {
		t.Fatalf("expected 3 smoke checks, got %d", len(smokes))
	}
	for _, c := range smokes {
		if !c.Passed {
			t.Errorf("smoke check %s failed: %s", c.Name, c.Detail)
		}
	}
}

func TestBuildStatusAndRender(t *testing.T) {
	cfg := BuildConfig(Customer{Name: "Pied Piper", Port: 0})
	rep, err := RunAgainstLiveService(cfg, []string{"health", "smoke"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ChecksTotal != 6 {
		t.Errorf("expected 6 checks, got %d", rep.ChecksTotal)
	}
	if rep.OverallHealth != "healthy" {
		t.Errorf("expected healthy, got %q (%d/%d)", rep.OverallHealth, rep.ChecksPassed, rep.ChecksTotal)
	}
	if rep.Metrics.RequestCount <= 0 || len(rep.Metrics.RequestSeries) != 24 {
		t.Errorf("metrics not populated: %+v", rep.Metrics)
	}
	if rep.Metrics.P95LatencyMs < rep.Metrics.P50LatencyMs {
		t.Errorf("p95 (%v) < p50 (%v)", rep.Metrics.P95LatencyMs, rep.Metrics.P50LatencyMs)
	}
	dir := t.TempDir()
	page := filepath.Join(dir, "status.html")
	if err := RenderStatusPage(rep, page); err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(page)
	for _, want := range []string{"Pied Piper", "Uptime", "Check results", "<polyline", "healthy"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

func TestMetricsReproducible(t *testing.T) {
	cfg := BuildConfig(Customer{Name: "Stark Industries"})
	a := buildMetrics(cfg, []float64{2.0, 3.0}, 6, 6)
	b := buildMetrics(cfg, []float64{2.0, 3.0}, 6, 6)
	if a.RequestCount != b.RequestCount {
		t.Errorf("metrics not reproducible: %d vs %d", a.RequestCount, b.RequestCount)
	}
}
