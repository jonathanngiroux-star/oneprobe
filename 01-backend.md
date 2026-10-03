# Project: Oneprobe
# Step 1 of 7: Programming the backend
# Role: execute this step only. Do not skip ahead. Do not add a subscription.

You are building **Oneprobe**, a solo-maintainable single-binary infrastructure monitor.
Replace the weekly correlation job behind Datadog / Grafana Cloud for DevOps and SRE leads at 100–500 person companies — not the entire observability catalog.

**Monetization override (all Hermes projects, including this one):**
- No subscriptions. No Cloud Starter / Pro / Enterprise invoices. No seat math. No usage bills. No "contact sales."
- No payment processor, no Stripe, no invoices.
- The only ask is optional donations to the project addresses:

Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Put both addresses in README, `/donate`, and every UI footer (GUI and TUI). Do not add other wallets without the maintainer saying so. Do not gate features on a donation.
The original Reality Seed map sold hosted tiers. That path is closed. Do not pretend this is MRR. Measure success as self-host deploys, migrate-dry-run usage, fixture quality, cold-start time, and on-chain donations — not ARR.

Read this brief as binding. If a request fights the wedge, this step, or the donation rule, refuse and name the trap.

## Sequence (do not reorder)

1. Programming the backend
2. Full audit and tests of the backend
3. Build a full GUI
4. Full audit and tests of the GUI
5. Build a full TUI
6. Full audit and tests of the TUI
7. Full audit and tests of the whole system

This file is **step 1**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

## Pitch

SRE leads will run a single binary that scrapes what Prometheus already scrapes, dry-runs a Datadog dashboard export, and alerts locally — then donate if it kills a weekly three-dashboard rotation. They will not get an Oneprobe invoice.

Named substitute (why it exists, not a price you charge): Datadog $1k–$10k/mo, or Prometheus + Grafana + Loki + Tempo.

## Stack

Go, SQLite (in-process series store only), Prometheus scrape, documented PromQL subset, local alerts, embedded GUI later, Bubble Tea TUI later.
Binary: `oneprobe` commands include `serve`, `migrate`, `query`, `alert`, `version`, `donate`.
Cold start: about 2 minutes, hard cap 15 minutes. Kubernetes is a bug.
License: MIT on core (scrape, store, query subset, alerts, migrate, GUI, TUI). No BSL cloud SKU.
GUI toolkit (step 3 only): [Wails](https://github.com/wailsapp/wails) v2. Desktop window. Go bindings. Not a browser-only admin site.
GOVERNANCE.md: DCO-only; donation addresses listed; no CLA surprise; 4/4 consensus + 30-day notice for license or wallet changes.

## Always out of scope

- Rebuilding Datadog (APM, RUM, synthetics, cloud-cost, security, 200 integrations)
- ClickHouse + Kafka + Redis + a four-service stack
- Kubernetes operator as the install path
- Hosted multi-tenant storage as a product
- Logs or traces before metrics + Datadog dashboard diff work
- Feature-gating query or alerting behind a donation

Fastest death: integration catalog before one binary + migrate dashboards + local alerts.

## Repo layout

```
oneprobe/
  IDEA.md
  AGENTS.md
  GOVERNANCE.md
  README.md
  LICENSE
  cmd/oneprobe/
  internal/{store,scrape,query,alert,migrate/datadog,serve,ui,tui}/
  testdata/fixtures/datadog/
  testdata/promql/
  Dockerfile
  docs/{donate.md,fidelity/datadog.md,query.md,migrate.md}
```

## This step

Program the backend. No GUI. No TUI. Tests that lock the backend contract are allowed; the full audit is step 2.

**Slice A — binary + store + scrape**
- Commands: serve, migrate, query, alert, version, donate
- `oneprobe donate` prints both addresses and exits 0
- SQLite schema: targets, samples (bounded retention), alert rules, alert state
- Prometheus text exposition scrape of one target
- `docker run`; measure cold start; write the number down

**Slice B — query subset**
- PromQL subset you document: selectors, rate/increase, sum/avg, comparison
- Fail CI if an undocumented function is advertised
- No GUI graph yet (that is step 3)

**Slice C — local alerts**
- Threshold rules on the query subset
- Notifier: log + webhook. No PagerDuty catalog.
- Self-host must fire an alert with no external control plane

**Slice D — Datadog migrate**
- Fixture: `testdata/fixtures/datadog/` (dashboard JSON + monitor JSON, anonymized)
- `oneprobe migrate --from=datadog --dry-run --format=json`
- Diff keys: `mapped` / `partial` / `unmapped` / `risks`
- Report stub in `docs/fidelity/datadog.md`
- CI: golden fixture → stable JSON. Do not fake 100% widget parity.

**Slice E — retention story**
- Document default retention and how to copy the SQLite file
- That is the operational story. You are not selling backups.

## Definition of done

- [ ] One binary, SQLite, documented `docker run` ≤15 min
- [ ] Cold start measured and published
- [ ] Scrape + store + query subset + local alert, no sidecars
- [ ] `oneprobe donate` prints both addresses exactly
- [ ] Datadog dry-run migrate + fidelity markdown from fixtures
- [ ] README states: free self-host, donations only, not a Datadog catalog
- [ ] No Stripe, no seats, no hosted retention SKU
- [ ] No logs platform, no traces, no Kubernetes requirement
- [ ] No GUI, no TUI (those are steps 3 and 5)

## How you work

- Every feature answers: does this help the wedge, deploy time, or donate visibility?
- If someone asks for a cloud SKU, point at `docs/donate.md` and stop.
- Output patches, commands, and file paths.

## First response required

1. Create/confirm `AGENTS.md` (this brief condensed, including donation-only and both addresses).
2. Scaffold the repo tree (no GUI/TUI implementation yet; empty packages ok).
3. Write target / sample / alert Go types + SQLite schema.
4. Implement `oneprobe version` and `oneprobe donate`.
5. Implement `oneprobe migrate --from=datadog --dry-run --format=json` as a stub that reads `testdata/fixtures/datadog/sample-dashboard.json` and prints `mapped` / `partial` / `unmapped` / `risks`.
6. Add `serve` with `/healthz` and `/donate` (plain text, both addresses).
7. Stop after the scrape + query subset + alert path exists and a smoke test passes. Do not start the GUI or TUI.
