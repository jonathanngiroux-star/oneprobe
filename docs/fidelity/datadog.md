# Datadog migration fidelity

Fidelity of the `oneprobe migrate --from=datadog --dry-run` report,
measured against the committed anonymized fixture
(`testdata/fixtures/datadog/sample-dashboard.json`). This page exists so
that a human can check every `partial` / `unmapped` classification and
no one can claim parity that is not there.

## Fixture: Web Overview (4 widgets)

| Widget | Datadog type | Classification | Why |
|---|---|---|---|
| Request rate | timeseries, `avg:system.cpu.user{host:web-1}` | mapped | Bare aggregation over one metric — translates to a PromQL-subset selector |
| Error ratio % | query_value, `anomalies(avg:...)` | partial | `anomalies()` has no PromQL-subset equivalent; the series renders without anomaly bands |
| Top hosts by memory | toplist, `top(avg:... by {host}, 5, ...)` | partial | `top()` is a Datadog function outside the subset; renders as a plain graph/table without ranking |
| Synthetic check | check_status | unmapped | Synthetics runner is out of scope for Oneprobe v0.1 |

Report totals: **1 mapped, 2 partial, 1 unmapped**, 3 risks.

## Known risks the report emits

- `query.anomalies` — anomaly bands do not survive translation; the raw
  series is shown instead.
- `widget.synthetics` — keep Datadog synthetic checks running elsewhere;
  Oneprobe has no synthetics runner.
- `tags.scope` — Datadog scope syntax (`host:web-1`) maps to PromQL
  label matchers, but label *names* must be verified against your actual
  exporters before cutover.

## What is not measured yet

- Datadog **monitor** (alert) exports — planned; classification will be
  added to this page in the same commit that ships the monitor fixture.
- Layout fidelity (dashboard size/position/grid) — Oneprobe's UI is a
  single graph view; layout is intentionally not preserved and is always
  classified `partial` for non-timeseries widget types.

## Rules for updating this page

- Numbers on this page must match the CI fixture gate exactly. If a
  classification changes, the gate, this page, and the fixture change in
  the same commit.
- Never round partial coverage up to "mapped." A silent mis-migration
  is worse than an honest `unmapped`.
