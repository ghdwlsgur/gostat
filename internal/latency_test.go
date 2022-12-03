package internal

import (
	"testing"
)

// Testing latency code.
func TestHttpLatency(t *testing.T) {
	results := make(chan Result)
	doneC := make(chan struct{})

	go GatherLatencies("https://www.naver.com", results, doneC)

	for r := range results {
		t.Log(r.URL, r.Latency)
	}
}
