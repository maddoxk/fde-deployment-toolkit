# Case Study: Cutting Customer Onboarding from 11 Days to Under 2 Hours at Northwind Logistics

> A simulated but realistic end-to-end forward-deployed engineering engagement, illustrating
> how `fde-deployment-toolkit` (`fdetk`) standardizes the deploy → verify → observe loop for a
> new enterprise customer.

## Customer & vertical

**Northwind Logistics** is a mid-market third-party logistics (3PL) provider operating regional
distribution centers across the EU. They buy a real-time shipment-tracking API from our platform
and embed it into their warehouse-management UI. Each new Northwind region is effectively a new
deployment of our sample tracking service, configured per-region (region code, replica count,
service tier).

- **Vertical:** B2B logistics / supply-chain SaaS
- **Integration surface:** a per-region containerized HTTP service (health/readiness/metrics/API)
- **Constraint:** Northwind's change-management policy requires every regional rollout to ship with
  a signed-off health report and a live status page their NOC can watch. No status page, no go-live.

## The problem (before)

Northwind's first two regions (`eu-west-1`, `eu-central-1`) were onboarded by hand. The forward-deployed
engineer wrote config by copy-pasting a previous customer's Kubernetes manifest, edited values in a
text editor, deployed, then verified the service with ad-hoc `curl` one-liners pasted into a Slack
thread. Observability was a screenshot of a Grafana panel emailed to Northwind's NOC.

Measured pain, averaged across the first two regions:

| Metric                                   | Before |
| ---------------------------------------- | -----: |
| Wall-clock time per region onboarding    | 11 days |
| Engineer-hours of hands-on config + verify | 9.5 hrs |
| Manifest defects caught in review        | 4 per region |
| Health verification                      | manual curl, inconsistent |
| Status artifact handed to customer       | static screenshot, stale on arrival |
| Rollbacks caused by missed readiness gap | 2 of 2 regions |

The two root causes: (1) configuration was hand-authored and drifted between regions, and (2) the
"is it healthy?" question was answered by a human running commands from memory, so the answer was
neither reproducible nor auditable.

## The approach

We built `fdetk` to collapse the whole onboarding loop into five composable subcommands that produce
**artifacts**, not Slack messages:

1. `scaffold` — renders `service.config.json` + a Kubernetes `deploy.yaml` from a small set of
   customer parameters (`--customer`, `--region`, `--tier`, `--replicas`, `--port`). Liveness and
   readiness probes are wired into the manifest automatically, eliminating the missed-readiness class
   of defect.
2. `deploy` — starts the sample service locally so the exact config that will ship can be exercised
   before it touches the cluster.
3. `healthcheck` — runs liveness, readiness, and metrics probes against the running service and
   exits non-zero if any fail (so it gates CI).
4. `smoke` — runs three end-to-end checks: an API echo round-trip, a JSON content-type assertion, and
   a metrics-counter-advances check.
5. `status` — runs all six checks against a freshly started service and emits a self-contained static
   status page (`status.html`) plus machine-readable `status.json`. This is the artifact Northwind's
   NOC bookmarks.

The key design decision: **the status page data is produced by actually running the checks**, never
hand-edited. The page Northwind sees is a faithful render of a real check run.

## Rollout phases

**Phase 0 — Pilot (1 region, `eu-west-1`).** We re-onboarded the existing `eu-west-1` region through
`fdetk` to validate parity with the hand-built config. The generated manifest matched the
hand-tuned one on every field except it *added* the readiness probe the human had forgotten.

**Phase 1 — Templatize tiers.** Northwind runs `standard` and `enterprise` tiers. We encoded the
tier into the image tag and replica defaults so a region's tier is a single flag, not a manifest edit.

**Phase 2 — CI gate.** `fdetk healthcheck --json` and `fdetk smoke --json` were wired into the
deployment pipeline. A non-zero exit blocks promotion, so a region cannot go live with a failing probe.

**Phase 3 — Customer-facing status page.** `fdetk status` output is published to a static bucket per
region. Northwind's NOC watches the live page; the `status.json` feeds their internal alerting.

**Phase 4 — Self-serve.** Northwind's own platform team now runs `fdetk scaffold` + `status` for new
regions and only escalates to us on a red check.

## Outcome (after)

| Metric                                   | Before | After | Change |
| ---------------------------------------- | -----: | ----: | -----: |
| Wall-clock time per region onboarding    | 11 days | < 2 hrs | **-98%** |
| Engineer-hours hands-on per region       | 9.5 hrs | 0.75 hr | **-92%** |
| Manifest defects caught in review        | 4 | 0 | **-100%** |
| Readiness-probe gaps reaching the cluster | 2 of 2 | 0 of 6 | eliminated |
| Health checks run per region (auditable) | ~0 consistent | 6 automated | — |
| Rollbacks from health regressions        | 2 | 0 | eliminated |
| Regions live after 1 quarter             | 2 | 8 | **4x** |

Across the next quarter Northwind brought **6 additional regions** live (8 total) with zero
health-related rollbacks. The standardized status page also cut the NOC's "is region X up?" tickets
to our on-call to roughly zero, because the answer was already on a page they controlled.

## What made it work

- **Artifacts over conversations.** Every step emits a file that can be reviewed, diffed, and archived.
- **The same code path produces the customer's status page and the CI gate**, so what the customer
  sees and what blocks a bad deploy can never disagree.
- **Zero runtime dependencies** (Go standard library only) meant the toolkit dropped into Northwind's
  air-gapped CI without a supply-chain review.

## Honest limitations

This is a simulated engagement and the sample service is an in-process stand-in for a real customer
workload; the metrics series on the status page are generated by a seeded model anchored on real
measured check latencies rather than scraped from production Prometheus. In a live deployment the
`status` command's metrics block would be wired to the customer's real metrics backend.
