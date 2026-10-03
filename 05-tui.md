# Project: Oneprobe
# Step 5 of 7: Build a full TUI
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

This file is **step 5**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

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

Build a full TUI. GUI stays. Backend stays. Do not add a second data model.

Full Bubble Tea (or equivalent) TUI in the same binary (`oneprobe tui`):
- Targets, scrape status, query prompt limited to the documented subset
- Alert list and fire state
- Migrate dry-run summary from a file path
- Donate screen with both addresses
- Mouse optional; keyboard complete
- Same data as the GUI; no second store

Use Bubble Tea or an equivalent in-repo TUI. `oneprobe tui` must run against the same SQLite (or stated data dir) as the GUI. Donate screen required. No mouse-only actions. No subscription UI.

## Definition of done

- [ ] TUI covers the workflows listed above
- [ ] Donate screen shows both addresses exactly
- [ ] README says how to launch the TUI
- [ ] GUI still works

## First response required

1. Add the TUI package and `oneprobe tui`.
2. Implement the screens against existing store APIs.
3. Document keys.
4. Stop before a new feature that is not on the list.
