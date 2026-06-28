# fde-deployment-toolkit (`fdetk`)

> A one-command deployment & customer-onboarding toolkit + live observability for forward-deployed engineers.

`fdetk` standardizes the **deploy → verify → observe** loop that a forward-deployed engineer (FDE)
runs for every new customer environment. Instead of hand-editing manifests and pasting `curl`
one-liners into Slack, every step emits a reproducible, auditable **artifact**: a generated config
and k8s manifest, a set of health/smoke check results, and a self-contained live status page rendered
from a **real** local service run.

**Live site:** https://maddoxk.github.io/fde-deployment-toolkit/
**Live status page:** https://maddoxk.github.io/fde-deployment-toolkit/status.html
**Case study:** https://maddoxk.github.io/fde-deployment-toolkit/case-study.html

The deployed status page is not hand-faked — it is generated in CI on every push by actually
starting the sample service and running all six checks against it (`fdetk status`).

## Why

For an FDE, customer onboarding is the same loop over and over: scaffold a per-customer service,
deploy it, prove it's healthy, and hand the customer something they can watch. Doing that by hand is
slow, drifty, and unauditable. `fdetk` turns that loop into one CLI so each onboarding is
reproducible and ships with a status artifact the customer's NOC can watch.

## Commands

| Command       | Action                                                        | Output |
| ------------- | ------------------------------------------------------------- | ------ |
| `scaffold`    | Render service config + k8s manifest from customer params     | `service.config.json`, `deploy.yaml`, sample service source |
| `deploy`      | Start (simulate deploying) the sample service locally         | running service + endpoint list |
| `healthcheck` | Liveness / readiness / metrics probes (non-zero exit on fail) | 3 check results — gates CI |
| `smoke`       | API echo round-trip, JSON content-type, metrics-advance       | 3 check results |
| `status`      | Run all checks against a live service & emit observability page| `status.html` + `status.json` |

## Quick start

```bash
git clone https://github.com/maddoxk/fde-deployment-toolkit
cd fde-deployment-toolkit
go build -o fdetk ./cmd/fdetk

./fdetk scaffold    --customer "Northwind Logistics" --tier enterprise --replicas 4
./fdetk deploy      --customer "Northwind Logistics"
./fdetk healthcheck --customer "Northwind Logistics"
./fdetk smoke       --customer "Northwind Logistics"
./fdetk status      --customer "Northwind Logistics" --out ./site
```

Common flags (all commands): `--customer`, `--slug`, `--env`, `--region`, `--tier`, `--port`, `--replicas`.
Run `./fdetk <command> --help` for the full set.

## Architecture

```
                 +------------------------+
   customer ---> |  scaffold              |  --> service.config.json
   params        |  (Slugify+BuildConfig) |      deploy.yaml (k8s manifest)
                 +-----------+------------+      sample_service.go.txt
                             |
                             v
                 +------------------------+
                 |  sample service        |  ephemeral httptest-style server:
                 |  (internal/toolkit)    |  /healthz /readyz /metrics /api/v1/echo
                 +-----------+------------+
                             |
              +--------------+--------------+
              v                             v
   +--------------------+        +--------------------+
   |  HealthChecks      |        |  SmokeChecks       |   real HTTP probes
   |  liveness/ready/   |        |  echo round-trip/  |   against the running
   |  metrics           |        |  content-type/     |   service
   +---------+----------+        |  metrics-advance   |
             |                   +---------+----------+
             +-----------+-------------------+
                         v
                 +------------------------+
                 |  BuildStatus           |  derive uptime, p50/p95/p99,
                 |  + RenderStatusPage    |  error rate, sparklines
                 +-----------+------------+
                             |
                  status.json + status.html  (self-contained, deployed to Pages)
```

- **`cmd/fdetk`** — CLI entrypoint and flag parsing (Go stdlib only).
- **`internal/toolkit`** — the engine: `scaffold.go` (config + manifest), `service.go` (the sample
  HTTP service), `checks.go` (health + smoke probes over real HTTP), `status.go` (metrics derivation
  + percentiles), `render.go` (status page template with inline SVG sparklines), `pipeline.go`
  (start service → run checks → build report).
- **`tools/buildsite`** — renders `CASE_STUDY.md` and the overview into the docs site (only
  third-party dependency: `goldmark` for Markdown).
- **`.github/workflows/pages.yml`** — builds the CLI, runs tests, runs a real `fdetk status` to
  produce the live status page, builds the docs site, and deploys to GitHub Pages.

## Observability output

`fdetk status` derives, from the actual check run against the live sample service:
uptime %, request volume, p50/p95/p99 latency, error rate, RPS, and request/latency sparklines,
plus the full table of six check results. These are written to `status.json` and rendered into a
self-contained `status.html` (no external assets — works at any base path).

## Case study

[`CASE_STUDY.md`](./CASE_STUDY.md) walks a simulated end-to-end Northwind Logistics engagement:
problem, approach, rollout, and outcome metrics — cutting per-region onboarding from **11 days to
under 2 hours** and hands-on engineer time by **~92%** across 8 regions in a quarter.

## Development

```bash
go build ./...     # build everything
go test ./...      # run the test suite
go run ./tools/buildsite .   # rebuild the docs site into ./site
```

Tests (`internal/toolkit/toolkit_test.go`) cover slugification, config defaults, scaffold artifact
output, that the sample service actually serves its endpoints, status build + render, and metric
reproducibility.

## License

MIT © 2026 Maddox Krape. See [LICENSE](./LICENSE).
