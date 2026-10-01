# Migrating from Datadog (dry-run)

`oneprobe migrate` reads a Datadog dashboard export and reports what
would translate to Oneprobe and what would not. In v0.1 it **only
reports** — it never writes anything, and it works without a Datadog
API key (you bring the export JSON yourself).

## Status

**Partially implemented.** The dry-run report on dashboard exports
works today; monitor (alert) export support is planned next. Run it
against the committed sample:

```
oneprobe migrate --from=datadog --dry-run --format=json \
  --input testdata/fixtures/datadog/sample-dashboard.json
```

Omit `--input` to default to the committed sample fixture.

## The report

`--format=json` prints a machine-stable report with four buckets —
every widget in the export lands in exactly one of them:

| Bucket | Meaning |
|---|---|
| `mapped` | Translates fully to an Oneprobe graph or rule |
| `partial` | Translates with caveats — the reason field says which |
| `unmapped` | No Oneprobe equivalent (e.g. synthetics checks) |
| `risks` | Migration hazards a human must review before cutover |

The JSON shape of this report is a CI contract: the committed fixture
must always produce the same report, and the CI job fails if it does not
— a mapping that silently changes classification fails the build.

## Fidelity

Per-fixture fidelity numbers and the reasoning behind every `partial`
classification live in [docs/fidelity/datadog.md](fidelity/datadog.md).
Oneprobe does not claim 100% widget parity and never will: the point of
the dry-run is to show the gap honestly before you cut over.
