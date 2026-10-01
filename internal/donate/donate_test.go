package donate

import "testing"

func TestEthereumUSDCAddress(t *testing.T) {
	const want = "0x85ee7E71f762d772599cbF1EC20E651B30657521"
	if EthereumUSDC != want {
		t.Errorf("EthereumUSDC = %q, want %q", EthereumUSDC, want)
	}
}

func TestBitcoinAddress(t *testing.T) {
	const want = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg"
	if Bitcoin != want {
		t.Errorf("Bitcoin = %q, want %q", Bitcoin, want)
	}
}

func TestAddressesListsBothWithChainLabels(t *testing.T) {
	addrs := Addresses()
	if len(addrs) != 2 {
		t.Fatalf("len(Addresses()) = %d, want 2", len(addrs))
	}
	if addrs[0].Chain != "Ethereum / USDC (ERC-20)" {
		t.Errorf("addrs[0].Chain = %q, want %q", addrs[0].Chain, "Ethereum / USDC (ERC-20)")
	}
	if addrs[0].Address != EthereumUSDC {
		t.Errorf("addrs[0].Address = %q, want %q", addrs[0].Address, EthereumUSDC)
	}
	if addrs[1].Chain != "Bitcoin" {
		t.Errorf("addrs[1].Chain = %q, want %q", addrs[1].Chain, "Bitcoin")
	}
	if addrs[1].Address != Bitcoin {
		t.Errorf("addrs[1].Address = %q, want %q", addrs[1].Address, Bitcoin)
	}
}

func TestPlainTextRendersBothAddressesWithAsk(t *testing.T) {
	text := PlainText()
	wantLines := []string{
		"Oneprobe is free to self-host.",
		"Ethereum / USDC (ERC-20): 0x85ee7E71f762d772599cbF1EC20E651B30657521",
		"Bitcoin: bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg",
	}
	for _, want := range wantLines {
		if !stringsContains(text, want) {
			t.Errorf("PlainText() missing line %q\ngot:\n%s", want, text)
		}
	}
}

func stringsContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
