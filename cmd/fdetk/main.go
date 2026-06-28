// Command fdetk is a one-command deployment / customer-onboarding toolkit.
//
// Subcommands:
//
//	scaffold     generate service config + manifest from customer params
//	deploy       start (simulate deploying) the sample service locally
//	healthcheck  run liveness/readiness/metrics probes against the service
//	smoke        run end-to-end smoke tests against the service
//	status       run all checks and emit a static status/observability page
//
// Run "fdetk <command> --help" for per-command flags.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/maddoxk/fde-deployment-toolkit/internal/toolkit"
)

const usage = `fdetk — FDE deployment & onboarding toolkit

Usage:
  fdetk <command> [flags]

Commands:
  scaffold     Generate service config + k8s manifest from customer params
  deploy       Start (simulate deploying) the sample service locally
  healthcheck  Run liveness/readiness/metrics probes against the service
  smoke        Run end-to-end smoke tests against the service
  status       Run all checks and emit a static status/observability page
  version      Print version

Run "fdetk <command> --help" for command-specific flags.
`

var version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scaffold":
		cmdScaffold(os.Args[2:])
	case "deploy":
		cmdDeploy(os.Args[2:])
	case "healthcheck":
		cmdChecks(os.Args[2:], []string{"health"}, "healthcheck")
	case "smoke":
		cmdChecks(os.Args[2:], []string{"smoke"}, "smoke")
	case "status":
		cmdStatus(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("fdetk", version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

func customerFlags(fs *flag.FlagSet) *toolkit.Customer {
	c := &toolkit.Customer{}
	fs.StringVar(&c.Name, "customer", "Acme Corp", "customer display name")
	fs.StringVar(&c.Slug, "slug", "", "DNS-safe slug (derived from --customer if empty)")
	fs.StringVar(&c.Environment, "env", "staging", "deployment environment")
	fs.StringVar(&c.Region, "region", "us-east-1", "cloud region")
	fs.StringVar(&c.Tier, "tier", "standard", "service tier (standard|premium|enterprise)")
	fs.IntVar(&c.Port, "port", 8080, "service port")
	fs.IntVar(&c.Replicas, "replicas", 2, "replica count")
	return c
}

func cmdScaffold(args []string) {
	fs := flag.NewFlagSet("scaffold", flag.ExitOnError)
	c := customerFlags(fs)
	out := fs.String("out", "./build", "output directory for generated artifacts")
	fs.Parse(args)

	cfg, err := toolkit.Scaffold(*c, *out)
	must(err)
	fmt.Printf("scaffolded %s for %q\n", cfg.ServiceName, cfg.Customer.Name)
	fmt.Printf("  -> %s\n", filepath.Join(*out, "service.config.json"))
	fmt.Printf("  -> %s\n", filepath.Join(*out, "deploy.yaml"))
	fmt.Printf("  -> %s\n", filepath.Join(*out, "sample_service.go.txt"))
}

func cmdDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	c := customerFlags(fs)
	wait := fs.Duration("wait", 600*time.Millisecond, "how long to keep the service up for inspection")
	fs.Parse(args)

	cfg := toolkit.BuildConfig(*c)
	svc, err := toolkit.NewSampleService(cfg)
	must(err)
	svc.Start()
	fmt.Printf("deploying %s (%s/%s, tier=%s, replicas=%d)\n",
		cfg.ServiceName, cfg.Customer.Environment, cfg.Customer.Region, cfg.Customer.Tier, cfg.Customer.Replicas)
	fmt.Printf("  service listening at %s\n", svc.BaseURL())
	fmt.Printf("  endpoints: %s %s %s %s\n", cfg.Endpoints.Health, cfg.Endpoints.Ready, cfg.Endpoints.Metrics, cfg.Endpoints.API)
	time.Sleep(*wait)
	fmt.Println("  deploy simulation complete (service stopped)")
}

func cmdChecks(args []string, kinds []string, name string) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	c := customerFlags(fs)
	asJSON := fs.Bool("json", false, "emit results as JSON")
	fs.Parse(args)

	cfg := toolkit.BuildConfig(*c)
	rep, err := toolkit.RunAgainstLiveService(cfg, kinds)
	must(err)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		must(enc.Encode(rep.Checks))
		return
	}
	fmt.Printf("%s: %s — %d/%d checks passed\n", name, cfg.ServiceName, rep.ChecksPassed, rep.ChecksTotal)
	for _, ck := range rep.Checks {
		status := "PASS"
		if !ck.Passed {
			status = "FAIL"
		}
		fmt.Printf("  [%s] %-26s %6.1fms  %s\n", status, ck.Name, ck.LatencyMs, ck.Detail)
	}
	if rep.ChecksPassed < rep.ChecksTotal {
		os.Exit(1)
	}
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	c := customerFlags(fs)
	outDir := fs.String("out", "./site", "output directory for the status page + json")
	pageName := fs.String("page", "status.html", "status page filename")
	fs.Parse(args)

	cfg := toolkit.BuildConfig(*c)
	rep, err := toolkit.RunAgainstLiveService(cfg, []string{"health", "smoke"})
	must(err)

	must(os.MkdirAll(*outDir, 0o755))
	jsonPath := filepath.Join(*outDir, "status.json")
	htmlPath := filepath.Join(*outDir, *pageName)
	must(toolkit.WriteJSON(jsonPath, rep))
	must(toolkit.RenderStatusPage(rep, htmlPath))

	fmt.Printf("status: %s — overall=%s, %d/%d checks passed\n",
		cfg.ServiceName, rep.OverallHealth, rep.ChecksPassed, rep.ChecksTotal)
	fmt.Printf("  uptime=%.2f%% reqs=%d errRate=%.2f%% p95=%.1fms\n",
		rep.Metrics.UptimePct, rep.Metrics.RequestCount, rep.Metrics.ErrorRatePct, rep.Metrics.P95LatencyMs)
	fmt.Printf("  -> %s\n  -> %s\n", htmlPath, jsonPath)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
