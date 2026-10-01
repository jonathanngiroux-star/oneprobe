# Oneprobe

**Category:** Single-binary infrastructure monitoring (open-core replication of a proven loop)
**Cohort:** Faster-to-cash if you copy the *wedge*, not the feature list
**Proof analog:** Anonymized solo monitor — ~$18.7k MRR by month 12, 2.3k self-hosts, 3.2% cloud conversion, 94% hosted revenue, 1.8% logo churn.

## Pitch

Self-hosted infra monitor (single binary, embedded SQLite, Prometheus-compatible) with a hosted tier from $29/mo. Replaces Datadog / Grafana Cloud at mid-market prices without a four-service Kubernetes stack.

## Weekly job

DevOps / SRE at 100–500 person companies correlate metrics, logs, and traces across 3+ dashboards, write alert rules by hand, and debug unknown-unknowns every on-call rotation.

## Who pays

SRE leads paying Datadog $1k–10k/mo or self-hosting Prometheus + Grafana + Loki + Tempo.

## Why incumbents fail

Datadog pricing is opaque. Grafana Cloud is a bundle of managed services. The “free” stack is four services plus ClickHouse/Kafka/Redis — “requires Kubernetes” for the buyer you want.

## Why this is still open

The conversion loop is proven. A second project has not cleanly replicated the *wedge*: one binary + hosted tier that sells storage, upgrades, backups, SSO — not a feature-gated clone.

## MVP (6–10 weeks)

- Go binary + embedded VictoriaMetrics or ClickHouse-lite
- PromQL compatibility
- `oneprobe migrate --from=datadog --dry-run` dashboard diff
- One-command Docker
- Hosted multi-tenant storage, SSO, retention policies
- Self-hosted parity on query/alert path

## License

MIT core. Hosted differentiation is operations and compliance, not ripped PromQL. BSL/ELv2 on multi-tenant control plane if you need the 12–18 month window.

## Pricing

- Cloud Starter — $29/mo (100GB metrics, 50GB logs)
- Cloud Pro — $299/mo (1TB, VPC, SSO, 13-month retention)

## 90-day evidence

- Weekly active self-host deploys (not stars)
- Conversion math published: deploys → paid
- Deploy time under 15 minutes (OpenReplay-class multi-service stacks fail this gate)
- SSO/audit in a form procurement can tick

## Living-income path

410 self-hosts → 15 cloud at $299 ≈ $4.5k MRR by month 6. Same physics as Plausible-class loops if churn stays ~2% logo / ~1% revenue.

## Investor “no”

Kubernetes required to self-host; cloud is only hosting; no migration diffs; deploy >30 minutes.

## Fastest death

Rebuilding Datadog. You will drown in integrations. The wedge is “one binary + migrate dashboards + stop paging yourself.”
