// Fetcher retrieves and parses one target's exposition document.
package scrape

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher scrapes one URL per call.
type Fetcher struct {
	client *http.Client
}

// NewFetcher builds a fetcher with the given per-request timeout.
func NewFetcher(timeout time.Duration) *Fetcher {
	return &Fetcher{client: &http.Client{Timeout: timeout}}
}

// Fetch retrieves the exposition document at url and parses it. It
// returns the parsed samples and the wall duration in milliseconds.
// Content-Type is ignored: the parser is the contract, and exporters in
// the wild are inconsistent about the header.
func (f *Fetcher) Fetch(url string) ([]Sample, int64, error) {
	start := time.Now()
	resp, err := f.client.Get(url)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("fetch %s: unexpected status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: read body: %w", url, err)
	}
	samples, err := Parse(string(body))
	if err != nil {
		return nil, 0, fmt.Errorf("fetch %s: %w", url, err)
	}
	return samples, time.Since(start).Milliseconds(), nil
}
