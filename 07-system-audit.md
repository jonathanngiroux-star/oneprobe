# Project: Oneprobe
# Step 7 of 7: Full audit and tests of the whole system
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

This file is **step 7**. Previous steps are assumed done. Later steps are out of scope until this step's definition of done is met.

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

Full audit and tests of the whole system: backend + GUI + TUI. Fix bugs. Do not add features.

System checks:
- `docker run` cold start re-measured
- GUI and TUI hit the same SQLite file without corruption
- Migrate JSON identical from CLI, GUI, and TUI on the golden fixture
- Donate addresses identical in CLI, GUI, TUI, README
- Kill -9 during scrape recovers
- No subscription, pricing, or Stripe strings anywhere in the tree

Also run:
- Backend suite, GUI suite, TUI suite, then one end-to-end script
- `docker run` from a clean tree
- Search the repo for Stripe, subscription, "contact sales", Cloud Pro — delete or rewrite to donate
- Confirm GOVERNANCE.md lists both addresses

Write `docs/audit/system.md` with bugs found and fixed. This is the release gate for v0.1.

## Definition of done

- [ ] All three surfaces agree on fixtures
- [ ] Donate addresses identical in CLI, GUI, TUI, README, GOVERNANCE.md
- [ ] Cold start still within the cap
- [ ] `docs/audit/system.md` written
- [ ] No new product surface
- [ ] v0.1 tag notes drafted in `docs/release-v0.1.md` (draft only)

## First response required

1. Run all suites and record failures.
2. Fix cross-surface bugs first.
3. Write `docs/audit/system.md` and the v0.1 draft notes.
4. Stop. Do not open the next product.
