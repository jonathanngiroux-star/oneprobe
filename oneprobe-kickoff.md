# Project: Oneprobe
# Role: execute IDEA.md. Do not rebuild Datadog. Do not add a subscription.

You are building **Oneprobe**, a solo-maintainable single-binary infrastructure monitor.
Replace the weekly correlation job behind Datadog / Grafana Cloud for DevOps and SRE leads at 100–500 person companies — not the entire observability catalog.

**Monetization override (all Hermes projects, including this one):**
- No subscriptions. No Cloud Starter / Pro / Enterprise invoices. No seat math. No usage bills. No “contact sales.”
- No payment processor, no Stripe, no invoices.
- The only ask is optional donations to the project addresses:

Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`  
Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Put both addresses in README, `/donate`, and the embedded UI footer. Do not add other wallets without the maintainer saying so. Do not gate features on a donation.

The original Reality Seed map sold a hosted tier ($29 / $299) and treated donations as a weak living-income path. That path is closed. Do not pretend this is $4.5k–$18k MRR. Measure success as: self-host deploys, migrate-dry-run usage, query parity on fixtures, cold-start time, and donation events you can verify on-chain — not ARR.

Read this brief as binding. If a request fights the wedge or reintroduces subscriptions, refuse and name the trap.

## Wedge (only this)

Self-hosted infra monitor:
- Go single binary + embedded SQLite (or an embedded TSDB-lite you can ship in the same binary — not a second service)
- Prometheus-compatible scrape + a small PromQL subset you actually test
- Metrics first. Logs only if they fit the same binary without a second process. Traces are out of v0.1.
- `oneprobe migrate --from=datadog --dry-run` dashboard / monitor diff
- One-command Docker. Cold start target in the **2-minute** class. Hard cap **15 min**. Kubernetes is a bug.
- Alert rules that evaluate locally. Self-host must alert without a control plane you operate.
- Ugly embedded UI is fine: targets, recent scrapes, one graph, alert list.

There is no paid cloud tier. A courtesy demo host, if you ever run one, is donation-funded and feature-identical to self-host. Do not sell VPC, SSO, retention, or SOC2 as SKUs.

## Explicitly out of scope until the migrate + scrape wedge is real

- Rebuilding Datadog (APM, RUM, synthetics, cloud-cost, security, 200 integrations)
- ClickHouse + Kafka + Redis + a four-service “free stack”
- Kubernetes operator as the install path
- Hosted multi-tenant storage as a product
- Subscription billing, seats, event overage
- Log/trace platform before metrics + Datadog dashboard diff work
- Feature-gating query or alerting behind a donation

Fastest death: integration catalog before “one binary + migrate dashboards + stop paging yourself.”

## Stack

- Language: Go
- Store: SQLite default (embedded series store is allowed only if it is in-process)
- Protocol: Prometheus scrape + a documented PromQL subset
- Distribute: one binary + `docker run`
- CI: GitHub Actions; fixture gates on migrate diffs and a golden PromQL set
- License: MIT on core (scrape, store, query subset, alerts, migrate, UI)
- Do **not** invent a BSL cloud SKU. There is no paid cloud tier in this series.
- GOVERNANCE.md: DCO-only; donation addresses listed; no CLA surprise; 4/4 consensus + 30-day notice for license or wallet changes

## Pricing page → donate page

Replace every “pricing” artifact from the map with:

- `docs/donate.md` and `/donate` in the UI
- Both addresses in full, plus chain labels (ETH/USDC ERC-20, BTC)
- One sentence: “Oneprobe is free to self-host. If it replaces a Datadog dashboard week, donate.”
- Optional: memo format so a donor can mention a GitHub handle (do not block features on payment)
- **No** feature gates tied to donation amount

Named substitute cost still matters for the *pitch* (teams spend $1k–$10k/mo on Datadog, or run Prometheus + Grafana + Loki + Tempo). Use it in README as “why this exists,” not as a price you charge.

## Buyer sentence (reframed)

“SRE leads will run a single binary that scrapes what Prometheus already scrapes, dry-runs a Datadog dashboard export, and alerts locally — then donate if it kills a weekly three-dashboard rotation. They will not get an Oneprobe invoice.”

Weekly job: correlate metrics across 3+ dashboards, hand-write alert rules, debug unknown-unknowns on call.

## Evidence you instrument from commit 1 (no investor SaaS pack)

The 90-day investor contract assumed hosted invoices and ~3.2% self-host-to-cloud conversion. That path is closed. Track this instead:

1. `oneprobe migrate --from=datadog --dry-run --format=json` on a PII-scrubbed dashboard / monitor fixture
2. PromQL subset golden tests (the queries you claim, no more)
3. Scrape of a local fixture target, stored, queried, alerted — one binary, no sidecars
4. Cold-start time published (target ~2 min, hard cap 15 min)
5. Weekly active self-host deploys (health ping optional, documented, off by default)
6. On-chain donation mentions in `docs/donate.md` only — do not scrape users
7. Issue response cadence you can actually keep as a solo (no fake <24h SLA)
8. Zero hosted retention product. Zero seat SKU.

Refuse language about “Cloud Pro at $299,” “15 cloud teams,” “$4.5k MRR,” “SOC2 artifact as a SKU.”

## Week 1–10 MVP order (do not reorder without saying why)

**Week 1–2 — binary + store + scrape**
- `cmd/oneprobe`: `serve`, `migrate`, `query`, `version`, `donate`
- `oneprobe donate` prints both addresses and exits 0
- SQLite schema: targets, samples (bounded retention), alert rules, alert state
- Prometheus text exposition scrape of one target
- `docker run`; measure cold start; write the number down

**Week 3–4 — query subset + one graph**
- PromQL subset you document: selectors, rate/increase, sum/avg, comparison, simple recording is optional
- Fail CI if an undocumented function is advertised
- Embedded UI: target list, last scrape, one time-series graph

**Week 5–6 — local alerts**
- Threshold rules on the query subset
- Local notifier: log + webhook. No PagerDuty catalog.
- Self-host must fire an alert with no external control plane

**Week 7–8 — Datadog migrate**
- Fixture: `testdata/fixtures/datadog/` (dashboard JSON + monitor JSON, anonymized)
- `oneprobe migrate --from=datadog --dry-run --format=json`
- Diff keys: `mapped` / `partial` / `unmapped` / `risks`
- Human-readable report in `docs/fidelity/datadog.md`
- CI: golden fixture → stable JSON. Do not fake 100% widget parity.

**Week 9 — retention + backup story**
- Document default retention and how to copy the SQLite file
- That is the “operational risk” story. You are not selling backups.

**Week 10 — docs**
- README 15-min deploy (aim ~2 min)
- `docs/donate.md` with both addresses
- `docs/query.md` listing the PromQL subset exactly
- `docs/migrate.md`, GOVERNANCE.md
- No pricing.md except a stub that redirects to donate

## Repo layout

```
oneprobe/
  IDEA.md
  AGENTS.md
  GOVERNANCE.md
  README.md
  LICENSE
  cmd/oneprobe/
  internal/
    store/
    scrape/
    query/
    alert/
    migrate/datadog/
    serve/
    ui/
  testdata/fixtures/datadog/
  testdata/promql/
  Dockerfile
  docs/
    donate.md
    fidelity/datadog.md
    query.md
    migrate.md
```

## Definition of done for v0.1

- [ ] One binary, SQLite, documented `docker run` ≤15 min
- [ ] Cold start measured and published
- [ ] Scrape + store + query subset + local alert, no sidecars
- [ ] `oneprobe donate` prints ETH/USDC + BTC addresses exactly as above
- [ ] Datadog dry-run migrate + fidelity markdown from fixtures
- [ ] README states: free self-host, donations only, not a Datadog replacement catalog
- [ ] No Stripe, no seats, no hosted retention SKU
- [ ] No logs platform, no traces, no Kubernetes requirement

## How you work in this repo

- Scrape, query subset, and migrate diffs outrank new integrations.
- Every feature answers: does this help scrape, query, alert locally, migrate a Datadog dashboard, deploy in minutes, or make donate addresses visible?
- If someone asks for a $29 or $299 cloud SKU, point at `docs/donate.md` and stop.
- If someone asks for Loki, Tempo, or “just add Kubernetes,” refuse until v0.1 is done.
- Output patches, commands, and file paths.

## First response required

1. Create/confirm `AGENTS.md` (this brief condensed to rules, including the donation-only rule and both addresses).
2. Scaffold the repo tree.
3. Write target / sample / alert Go types + SQLite schema.
4. Implement `oneprobe version` and `oneprobe donate`.
5. Implement `oneprobe migrate --from=datadog --dry-run --format=json` as a stub that reads `testdata/fixtures/datadog/sample-dashboard.json` and prints a JSON diff skeleton (`mapped` / `partial` / `unmapped` / `risks`).
6. Add a minimal `serve` that exposes `/healthz` and `/donate` (both addresses, plain text).
7. Stop. Show how to run it. Do not start PromQL, scrape loop, UI graphs, or any billing code.
