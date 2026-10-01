# Oneprobe — query subset

Oneprobe stores scraped samples and evaluates a deliberately small,
documented subset of PromQL. This page is the contract: if a query form
is not listed here, Oneprobe does not support it, and any function
advertised in docs or code that is not listed here fails CI.

## Status

**Planned.** The query engine lands in weeks 3–4 of the MVP order (see
AGENTS.md). The currently implemented surface is the storage schema
(`internal/store`) and the migrate dry-run report; there is no query
parser in the tree yet, and `oneprobe query` is not implemented. This
document will be updated in the same commit that introduces the parser,
and CI will pin the supported set to a golden list.

## The subset (target for v0.1)

Selectors:

- `metric_name` — all series for a metric
- `metric_name{label="value"}` — exact label match
- `metric_name{label=~"regex"}` — regex label match

Functions (over ranges):

- `rate(metric_name[5m])`
- `increase(metric_name[5m])`

Aggregations:

- `sum(...)`, `avg(...)`, `max(...)`, `min(...)` over label groups

Comparisons:

- `expr > n`, `expr >= n`, `expr < n`, `expr <= n`, `expr == n`,
  `expr != n`

Explicitly **not** in the v0.1 subset: `histogram_quantile`, subqueries,
`offset`, `@` modifiers, `absent`, `clamp`, label manipulation functions
(`label_replace` etc.), binary operations between two series, and
anything the scrape path has not been tested against.

## Why so small

A query subset you actually test beats a large one you believe works.
The Datadog migrate dry-run reports honestly when a dashboard query
falls outside this set (`partial` / `unmapped`) rather than silently
mis-translating it — see docs/migrate.md.
