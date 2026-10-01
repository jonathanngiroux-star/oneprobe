# GOVERNANCE.md

## Project

Oneprobe is a single-binary infrastructure monitor (Go + embedded
SQLite). It is maintained by a solo maintainer with lightweight,
consensus-based governance for the few decisions that are bigger than
one commit.

## License

- The entire repository is MIT (see LICENSE).
- There is no CLA. Contributions are accepted under the Developer
  Certificate of Origin (DCO): every commit must be signed off
  (`git commit -s`), certifying you wrote or have the right to submit
  the work.

## Decisions requiring maintainer consensus

The following changes require **4/4 maintainer consensus plus 30 days
of public notice** in the repository before taking effect:

1. Any change to the project license.
2. Any change to the donation addresses listed in README.md,
   docs/donate.md, the `oneprobe donate` command output, or the /donate
   endpoint. (The same two addresses must appear in all four places; a
   wallet change is a coordinated change, not a one-file edit.)
3. Adding any additional donation wallet beyond the two listed.

Until the maintainer count reaches four, consensus means the solo
maintainer plus a 30-day public notice in the form of a GitHub issue
pinned to the repository.

## Non-negotiables

These are standing project decisions and are not revisited by ordinary
contribution:

- Oneprobe has no paid tier, no subscriptions, no seat pricing, no
  payment processor, and no feature gated on a donation. The project is
  funded by donations to the addresses above.
- The self-hosted deployment path is a single Go binary with embedded
  SQLite. Kubernetes is not the install path. A change that requires a
  second long-running service for core operation is out of scope for
  v0.1 and needs consensus to enter scope at all.

## Donation addresses

- Ethereum / USDC (ERC-20): `0x85ee7E71f762d772599cbF1EC20E651B30657521`
- Bitcoin: `bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg`

Oneprobe is free to self-host. If it replaces a Datadog dashboard week,
donate. Donations are optional and gate no functionality.
