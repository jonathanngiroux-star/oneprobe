# Alerts

Oneprobe evaluates alert rules locally, in the same binary that scrapes
and stores. There is no external alert controller, no Alertmanager
requirement, and no control plane you have to operate: if the process is
running, your alerts are being evaluated.

## Rules

A rule is:

| Field | Meaning |
|---|---|
| `name` | Unique; re-POSTing the same name updates the rule |
| `expr` | Any expression from the [query subset](query.md) |
| `op`, `value` | Threshold comparison (`up < 1`) |
| `for_secs` | The condition must hold this long before firing (0 = immediate) |
| `webhook_url` | Optional; POSTed on state transitions |
| `enabled` | Disabled rules are never evaluated |

The rule's condition is "some series of `expr` violates `op value`" —
specifically the most-violating value drives the rule. An expression
with **no series** (metric absent, no data yet) is *not* a violation:
an absent metric resolves rather than firing. A 0 you did not scrape is
not a 0 you monitored.

## State machine

`resolved → pending → firing → resolved`

- With `for_secs = 0`: violating immediately fires.
- With `for_secs = n`: the first violating evaluation moves the rule to
  `pending`; it fires only when the condition has held for `n` seconds
  continuously. The condition clearing at any point resolves the rule.
- Transitions (not every evaluation) update `changed_at`.

State is persisted in SQLite (`alert_state`), so a restart does not
lose firing state or pending timers.

## Notifiers

- **Log** — every transition is logged:
  `alert: "host-down" FIRING (up < 1, value=0 at 2025-10-01T08:00:00Z)`.
- **Webhook** — if a rule has a `webhook_url`, state transitions POST:
  ```json
  {
    "rule": "host-down", "expr": "up", "op": "<", "threshold": 1,
    "state": "firing", "value": 0, "at": "2025-10-01T08:00:00Z"
  }
  ```
  A failing or slow webhook is logged and skipped — it never blocks
  evaluation of the other rules. There is no PagerDuty catalog; point
  the webhook at whatever you already run.

## API

- `GET /api/alerts` — rules joined with live state
- `POST /api/alerts` — create/update: `{"name":"host-down","expr":"up","op":"<","value":1,"for_secs":0,"enabled":true}`
- `DELETE /api/alerts/{id}` — remove (state cascades)

The engine evaluates every 15 seconds (every 1s tick re-reads rules,
so a new rule is picked up without a restart).
