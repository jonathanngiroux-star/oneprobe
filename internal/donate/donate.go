// Package donate holds Oneprobe's donation addresses. Oneprobe has no
// paid tier, no subscriptions, and no feature gates — donations are the
// only way to support development, and nothing is ever gated on them.
package donate

import "strings"

// EthereumUSDC and Bitcoin are the project's only donation addresses.
// Adding or changing a wallet requires 4/4 maintainer consensus and
// 30 days of public notice (see GOVERNANCE.md).
const (
	EthereumUSDC = "0x85ee7E71f762d772599cbF1EC20E651B30657521"
	Bitcoin      = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg"
)

// Address is one donation destination with its chain label.
type Address struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}

// Addresses lists every donation address, in the order the project
// documents them (Ethereum/USDC first, then Bitcoin).
func Addresses() []Address {
	return []Address{
		{Chain: "Ethereum / USDC (ERC-20)", Address: EthereumUSDC},
		{Chain: "Bitcoin", Address: Bitcoin},
	}
}

// PlainText renders the donation block used by `oneprobe donate`,
// /donate, README, and docs/donate.md. Ask is one sentence: donations
// are optional and gate nothing.
func PlainText() string {
	var b strings.Builder
	b.WriteString("Oneprobe is free to self-host. If it replaces a Datadog dashboard week, donate.\n\n")
	b.WriteString("Ethereum / USDC (ERC-20): " + EthereumUSDC + "\n")
	b.WriteString("Bitcoin: " + Bitcoin + "\n")
	return b.String()
}
