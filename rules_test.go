package main

import (
	"slices"
	"testing"
)

func TestSelectorsOfDropsModifiersAndKeepsMatchers(t *testing.T) {
	got, err := selectorsOf(`rate(a_total{code=~"5.."}[5m] offset 10m) / b @ 1609746000 > c offset -5m`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`a_total{code=~"5.."}`, "b", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSelectorsOfLeavesOutTheGeneratedRecord(t *testing.T) {
	got, err := selectorsOf(`count by (job, instance) (selector_disappeared_series and on (job, instance) up == 1) > 0` +
		` or {__name__=~"selector_.+"}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"up"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractSelectorsIgnoresRecordRules(t *testing.T) {
	got, err := extractSelectors([]byte(`
groups:
  - name: g
    rules:
      - record: r
        expr: recorded_only
      - alert: A
        expr: alerted > 0
`))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"alerted"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractSelectorsFailsOnUnparsableExpr(t *testing.T) {
	got, err := extractSelectors([]byte(`
groups:
  - name: g
    rules:
      - alert: Good
        expr: ok > 0
      - alert: Bad
        expr: "broken{"
`))
	if err == nil {
		t.Errorf("got %q and no error, want an error", got)
	}
}
