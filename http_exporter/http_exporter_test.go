package main

import (
	"testing"
)

func TestVisit(t *testing.T) {
	c := newHTTPStatsCollector("https://example.com", 10)
	err := c.visit()
	if err != nil {
		t.Fatal(err)
	}
}
