# Oneprobe — query subset

Oneprobe stores scraped samples and evaluates a deliberately small,
documented subset of PromQL. This page is the contract: if a query form
is not listed here, Oneprobe does not support it, and any function
advertised in docs or code that is not listed here fails CI.

## Status

**Implemented.** The engine lives in `internal/query` and is exposed at
`GET /api/query?expr=<expression>`. Every form below is covered by
unit tests (`internal/query/parse_test.go`, `eval_test.go`); the parser
refuses anything outside this set by name.

## Selectors

- `metric_name` — every series of that metric (last sample per series
  within a 5-minute lookback)
- `metric_name{label="value"}` — exact label match
- `metric_name{label=~"regex"}` — regex label match
- Multiple matchers AND together; matcher order is irrelevant (sorted
  internally)

## Range functions

- `rate(metric_name[5m])` — (last − first) / seconds-span over the range
- `increase(metric_name[5m])` — last − first over the range

Durations: `ms s m h d w`. A single sample in the range evaluates to 0
(honest "no signal" for a just-started counter).

## Aggregations

- `sum(...)`, `avg(...)`, `max(...)`, `min(...)` — one global group
- `sum by (label) (...)`, `avg by (a,b) (...)` — group by label names
- Aggregations compose with range functions:
  `sum by (code) (rate(http_requests_total[5m]))`

## Comparisons

- `expr > n`, `expr >= n`, `expr < n`, `expr <= n`, `expr == n`,
  `expr != n` — filters the vector to series satisfying the comparison

## Evaluation semantics

- Instant evaluation: a bare selector yields the **last** sample of each
  matching series within the lookback window.
- `rate`/`increase` use the first and last samples in their range.
- Aggregations with `by` group on label values; without `by` they
  aggregate all series into one.
- An empty result is a valid result (`{"series": []}`).

## Explicitly not in the subset

`histogram_quantile`, subqueries, `offset`, `@` modifiers, `absent`,
`clamp`, label manipulation (`label_replace` etc.), binary operations
between two series, and `count()`. The parser names the rejected
function and prints the supported set. A query using an unsupported
form is a 400 from `/api/query?expr=`, never a silent empty result.

## Why so small

A query subset you actually test beats a large one you believe works.
The Datadog migrate dry-run reports honestly when a dashboard query
falls outside this set (`partial` / `unmapped`) rather than silently
mis-translating it — see [docs/migrate.md](migrate.md).
