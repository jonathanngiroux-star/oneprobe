# Oneprobe

Single-binary infrastructure monitoring: scrape Prometheus-format
targets, store samples in embedded SQLite, query a small documented
PromQL subset, evaluate alert rules locally, and dry-run a migration
report against Datadog dashboard exports — one `docker run`, no
Kubernetes, no sidecars.

Oneprobe is not a Datadog replacement catalog. It is the small,
self-contained monitor that covers the metrics-and-alerting core a team
actually looks at every day, plus an honest report of what a Datadog
dashboard would lose if you moved it here.

**Free to self-host. Donations only — no paid tier, no subscriptions,
nothing is gated on money.**

## Why this exists

Teams either pay Datadog $1k–10k/month or run the "free" stack:
Prometheus + Grafana + Loki + Tempo, four services and a Kubernetes
dependency. Oneprobe is the third option: one binary with the metrics
core, for operators who want the 90% they use without the two
infrastructure options above it.

## Status

Currently working:

- `oneprobe version`, `oneprobe donate`
- SQLite storage layer (targets, samples, alert rules, alert state)
- `oneprobe migrate --from=datadog --dry-run --format=json` —
  dashboard-export diff with mapped / partial / unmapped / risks, run on
  the committed sample fixture or your own export
- `oneprobe serve` — HTTP server with `/healthz` and `/donate`

Planned (MVP order, see AGENTS.md): Prometheus scrape loop, the
documented PromQL subset ([docs/query.md](docs/query.md)), local
threshold alerts, monitor-export migration, the embedded UI.

## Quickstart

Build and run (Go 1.27+):

```
go build -o oneprobe ./cmd/oneprobe
./oneprobe serve --db /tmp/oneprobe.db --http 127.0.0.1:8080
```

Or with Docker (image built from this repo):

```
docker build -t oneprobe .
docker run -p 8080:8080 -v oneprobe-data:/data oneprobe
```

Then check liveness and the donation page:

```
curl http://127.0.0.1:8080/healthz   # -> ok
curl http://127.0.0.1:8080/donate    # -> donation addresses, plain text
```

Measured cold start (image already built): **0.87s** from `docker run`
to the first successful `/healthz` on a fresh volume (x86-64, Docker
29). With `go build` included from a clean checkout, the full path is
minutes-class, well inside the 15-minute hard cap.

Try the Datadog dry-run on the committed sample (no Datadog account
needed — you bring an export JSON yourself in real use):

```
./oneprobe migrate --from=datadog --dry-run --format=json
```

See [docs/migrate.md](docs/migrate.md) for the report format and
[docs/fidelity/datadog.md](docs/fidelity/datadog.md) for what the sample
fixture maps to.

## Commands

| Command | What it does |
|---|---|
| `oneprobe serve` | HTTP server: `/healthz`, `/donate` (UI as it lands) |
| `oneprobe migrate --from=datadog --dry-run` | Print a migration diff report for a Datadog dashboard export |
| `oneprobe version` | Print binary name and version |
| `oneprobe donate` | Print the donation addresses |

## Documentation

- [docs/migrate.md](docs/migrate.md) — the dry-run report and its contract
- [docs/query.md](docs/query.md) — the exact PromQL subset (the contract; CI enforces it)
- [docs/fidelity/datadog.md](docs/fidelity/datadog.md) — honest fidelity numbers from the fixture
- [docs/donate.md](docs/donate.md) — donations
- [GOVERNANCE.md](GOVERNANCE.md) — DCO, license, wallet-change consensus
- [AGENTS.md](AGENTS.md) — working rules for contributors and agents

## Donations

Oneprobe is free to self-host under MIT. If it replaces a Datadog
dashboard week, donate — optional, and no feature is ever gated on it:

- Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
- Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Details: [docs/donate.md](docs/donate.md). For pricing questions: there
isn't any — [docs/pricing.md](docs/pricing.md).

## License

MIT — see [LICENSE](LICENSE). Contributions follow the DCO
(`git commit -s`), no CLA. See [GOVERNANCE.md](GOVERNANCE.md).
