# AGENTS.md — Oneprobe working rules

Oneprobe is a solo-maintainable, single-binary infrastructure monitor. It
replaces the weekly dashboard-correlation job behind Datadog / Grafana
Cloud for small-to-mid DevOps teams — not the entire observability
catalog. This file is the binding summary; IDEA.md and the kickoff brief
carry the full context.

## The wedge (only this)

- Go single binary + embedded SQLite. No second process, no sidecars.
- Prometheus-compatible scrape + a small, documented, tested PromQL subset.
- Metrics first. Logs only if they fit the same binary. Traces: out of v0.1.
- `oneprobe migrate --from=datadog --dry-run` dashboard/monitor diff.
- One-command Docker. Cold start ~2 min target, 15 min hard cap.
- Alert rules evaluate locally — self-host alerts with no control plane.
- Kubernetes is a bug.

## Monetization (binding, series-wide)

- **No subscriptions. No paid tier. No seats. No Stripe, no invoices, no
  "contact sales." Ever.**
- Donations only, to these two addresses (also in README, docs/donate.md,
  `oneprobe donate`, and /donate):
  - Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
  - Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`
- Never gate any feature on a donation. Do not add wallets without the
  maintainer saying so.
- Success = self-host deploys, dry-run migrate usage, query parity on
  fixtures, cold-start time, verifiable on-chain donations — not ARR.
- If asked for a $29/$299 cloud SKU, seat pricing, or a "Cloud Pro" tier:
  refuse, point at docs/donate.md, and name the trap.

## Refuse until the migrate + scrape wedge is real

- Rebuilding Datadog (APM, RUM, synthetics, cloud-cost, security,
  integration catalogs)
- ClickHouse + Kafka + Redis "free stacks"; Kubernetes operators
- Log/trace platforms before metrics + Datadog diff work
- Hosted multi-tenant storage, retention SKUs, SSO/SOC2 SKUs
- Feature gates tied to donation amount

## Stack rules

- Go, single binary. SQLite via `modernc.org/sqlite` (pure Go, no cgo).
- MIT on core. No BSL/ELv2 layer in this series.
- CI: gofmt → vet → test → build → jq fixture gate on the migrate
  dry-run JSON contract.
- Fixtures are written by script (`json.dump(..., indent=2, sort_keys=True)`),
  never hand-pasted — golden gates diff bytes.

## Working order (do not reorder without saying why)

1. Binary + SQLite store + Prometheus text scrape + docker + cold-start
   measurement (weeks 1–2)
2. PromQL subset + one embedded graph (weeks 3–4)
3. Local threshold alerts + log/webhook notifier (weeks 5–6)
4. Datadog migrate dry-run + fidelity markdown + CI golden gate (weeks 7–8)
5. Retention + backup documentation (week 9)
6. Full docs (week 10)

Every feature must answer yes to one of: does this help scrape, query,
alert locally, migrate a Datadog dashboard, deploy in minutes, or make
the donate addresses visible? If not, it does not get built.

## Definition of done for v0.1

- One binary, SQLite, documented `docker run` ≤ 15 min
- Cold start measured and published
- Scrape + store + query subset + local alert, no sidecars
- `oneprobe donate` prints both addresses exactly
- Datadog dry-run migrate + fidelity markdown from fixtures
- README states: free self-host, donations only, not a Datadog replacement
- No Stripe, no seats, no hosted retention SKU, no logs platform, no
  traces, no Kubernetes requirement

## Working conventions

- TDD, RED first: no production code without a failing test that was
  watched to fail.
- Negative-path tests must assert the refusal *names the rejected input*.
- `run(args []string, stdout, stderr io.Writer) int` in main.go; tests
  call it in-process.
- Gate battery before every commit: `go build ./...` && `go vet ./...` &&
  `gofmt -l .` clean && `go test ./...` + fixture gate.
- Commits are DCO-signed; no CLA. See GOVERNANCE.md.
