package ui

import (
	"strings"
	"testing"
)

// TestPageEscapeFunctionMapsToHTMLEntities pins the esc() contract at
// the source level: every dangerous byte must map to an HTML entity
// built from JS unicode escapes (so no tooling layer can decode them
// back into raw bytes). The first version of this page shipped a
// NO-OP esc() — it "escaped" & < > " to themselves — which piped
// remote scrape-error text straight into innerHTML; a second version
// had a raw three-quote sequence that broke the whole script block.
// This test exists so neither can recur.
func TestPageEscapeFunctionMapsToHTMLEntities(t *testing.T) {
	page := Page()
	mappings := []string{
		`.replace(/&/g, "\u0026amp;")`,
		`.replace(/</g, "\u0026lt;")`,
		`.replace(/>/g, "\u0026gt;")`,
		`.replace(/"/g, "\u0026quot;")`,
	}
	for _, m := range mappings {
		if !strings.Contains(page, m) {
			t.Errorf("esc() missing entity mapping %q — unescaped bytes reach innerHTML", m)
		}
	}
	// The no-op forms must be gone.
	noops := []string{
		`.replace(/&/g, "&")`,
		`.replace(/</g, "<")`,
		`.replace(/>/g, ">")`,
		`.replace(/"/g, "\"")`,
	}
	for _, n := range noops {
		if strings.Contains(page, n) {
			t.Errorf("esc() still contains no-op/broken mapping %q", n)
		}
	}
	// & must be replaced FIRST, or the other entities double-escape.
	ampIdx := strings.Index(page, `.replace(/&/g, "\u0026amp;")`)
	ltIdx := strings.Index(page, `.replace(/</g, "\u0026lt;")`)
	if ampIdx > ltIdx {
		t.Errorf("& must be replaced before < (amp at %d, lt at %d)", ampIdx, ltIdx)
	}
}

// TestPageScriptHasBalancedQuotes: the embedded <script> must contain
// no `"""` sequence — the signature of a broken quote mapping — so a
// syntactically dead script cannot ship again.
func TestPageScriptHasBalancedQuotes(t *testing.T) {
	page := Page()
	if strings.Contains(page, `"""`) {
		t.Error("script contains a raw \"\"\" sequence — the script block cannot parse")
	}
}
