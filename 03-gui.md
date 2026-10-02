# Project: Oneprobe
# Step 3 of 7: Build a full GUI
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

This file is **step 3**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

## Pitch

SRE leads will run a single binary that scrapes what Prometheus already scrapes, dry-runs a Datadog dashboard export, and alerts locally — then donate if it kills a weekly three-dashboard rotation. They will not get an Oneprobe invoice.

Named substitute (why it exists, not a price you charge): Datadog $1k–$10k/mo, or Prometheus + Grafana + Loki + Tempo.

## Stack

Go, SQLite (in-process series store only), Prometheus scrape, documented PromQL subset, local alerts, embedded GUI later, Bubble Tea TUI later.
Binary: `oneprobe` commands include `serve`, `migrate`, `query`, `alert`, `version`, `donate`.
Cold start: about 2 minutes, hard cap 15 minutes. Kubernetes is a bug.
License: MIT on core (scrape, store, query subset, alerts, migrate, GUI, TUI). No BSL cloud SKU.
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

Build a full GUI on the finished backend. Do not redesign the backend except for a thin HTTP handler the GUI needs. Do not start the TUI.

Full local web GUI served by the same binary (no separate frontend server):
- Target list, add/remove target, last scrape error
- Time-series graph for the documented query subset
- Alert rule editor (threshold on a saved query) and alert history
- Datadog migrate dry-run: upload fixture or paste JSON, show mapped/partial/unmapped
- Donate page with both addresses, copy buttons, no payment form
- Retention and backup note (copy the SQLite file)
Ugly is fine. Accessible labels, keyboard path for every primary action, no external CDN required to function.

Serve the GUI from the same binary. No separate Node server required at runtime. No external CDN required for the app to function. No subscription UI, no pricing table, no Stripe.

## Definition of done

- [ ] Every backend wedge action is reachable in the GUI
- [ ] Donate page shows both addresses exactly
- [ ] README says how to open the GUI
- [ ] No TUI yet
- [ ] No new billing

## First response required

1. Add the GUI package and routes.
2. Implement the screens listed above against existing APIs.
3. Add a smoke script or note in README.
4. Stop before the TUI and before visual polish that does not change a workflow.
