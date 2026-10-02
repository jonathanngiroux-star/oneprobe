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
- SQLite storage layer (targets, samples, scrapes, alert rules, alert state)
- Prometheus text scrape loop (register a target, it scrapes on its interval)
- The documented PromQL subset ([docs/query.md](docs/query.md)):
  selectors, matchers, `rate`/`increase`, `sum`/`avg`/`max`/`min` with
  `by`, comparisons — at `GET /api/query?expr=`
- Local threshold alerts with a `for` duration, persisted state, and
  log + webhook notifiers ([docs/alerts.md](docs/alerts.md))
- `oneprobe migrate --from=datadog --dry-run --format=json` —
  dashboard-export diff with mapped / partial / unmapped / risks, run on
  the committed sample fixture or your own export
- `oneprobe serve` — HTTP server with `/healthz`, `/donate`, the API,
  and the embedded GUI (targets with last-error column, scrapes, alert
  rules with history, the query graph, the Datadog dry-run screen with
  paste/file input, the retention note)
- `POST /api/migrate/datadog` — the dry-run report from a pasted
  dashboard export (the GUI's path; the CLI reads a file)

Planned (MVP order, see AGENTS.md): monitor-export migration, retention
sweep automation, README deploy timing refresh.

## Quickstart

Build and run (Go 1.27+):

```
go build -o oneprobe ./cmd/oneprobe
./oneprobe serve --db /tmp/oneprobe.db --http 127.0.0.1:8080
```

**Open the GUI:** point a browser at `http://127.0.0.1:8080/` —
targets, recent scrapes with errors, the query graph, alert rules with
history, the Datadog dry-run screen, and the retention note are all
there. The donate page is at `http://127.0.0.1:8080/donate`.

Or with Docker (image built from this repo):

```
docker build -t oneprobe .
docker run -p 8080:8080 -v oneprobe-data:/data oneprobe
```

Then check liveness and the donation page:

```
curl http://127.0.0.1:8080/healthz   # -> ok
curl http://127.0.0.1:8080/donate    # -> donate page (HTML)
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
| `oneprobe serve` | HTTP server: `/healthz`, `/donate`, API, embedded UI |
| `oneprobe migrate --from=datadog --dry-run` | Print a migration diff report for a Datadog dashboard export |
| `oneprobe version` | Print binary name and version |
| `oneprobe donate` | Print the donation addresses |

## Documentation

- [docs/migrate.md](docs/migrate.md) — the dry-run report and its contract
- [docs/query.md](docs/query.md) — the exact PromQL subset (the contract; CI enforces it)
- [docs/alerts.md](docs/alerts.md) — alert rules, state machine, notifiers
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
