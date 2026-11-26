package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
	"go.yaml.in/yaml/v3"
)

func TestDisappearedExprLooksBackSeenWithinOnTheLeftAndAbsentForOnTheRight(t *testing.T) {
	expr := disappearedExpr(`a_total{code=~"5.."}`, model.Duration(48*time.Hour), model.Duration(time.Hour))
	ast, err := exprParser.ParseExpr(expr)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	var ranges []time.Duration
	parser.Inspect(ast, func(node parser.Node, _ []parser.Node) error {
		if ms, ok := node.(*parser.MatrixSelector); ok {
			ranges = append(ranges, ms.Range)
		}
		return nil
	})
	if len(ranges) != 2 || ranges[0] != 48*time.Hour || ranges[1] != time.Hour {
		t.Errorf("%s: ranges %v, want [48h 1h]", expr, ranges)
	}
}

func TestRunWritesOneRulePerUniqueSelectorInStableOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "first.yml", `
groups:
  - name: g
    rules:
      - alert: A
        expr: zeta > 0 and shared > 0
`)
	writeFile(t, dir, "second.yml", `
groups:
  - name: h
    rules:
      - alert: B
        expr: shared offset 5m > 0 and alpha > 0
`)

	var out bytes.Buffer
	if err := run([]string{"--rule-files", filepath.Join(dir, "*.yml")}, &out); err != nil {
		t.Fatal(err)
	}
	var got generatedFile
	if err := yaml.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Groups) != 1 {
		t.Fatalf("got %+v, want one group", got.Groups)
	}
	var selectors []string
	for _, r := range got.Groups[0].Rules {
		selectors = append(selectors, r.Labels[selectorLabel])
	}
	want := []string{"alpha", "shared", "zeta"}
	if len(selectors) != len(want) {
		t.Fatalf("got %q, want %q", selectors, want)
	}
	for i := range want {
		if selectors[i] != want[i] {
			t.Errorf("got %q, want %q", selectors, want)
		}
	}
}

func TestRunWritesNothingWhenARuleFileIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good.yml", `
groups:
  - name: g
    rules:
      - alert: A
        expr: ok > 0
`)
	if err := os.Symlink(filepath.Join(dir, "nowhere"), filepath.Join(dir, "unreadable.yml")); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := run([]string{"--rule-files", filepath.Join(dir, "*.yml")}, &out); err == nil {
		t.Error("got no error, want one")
	}
	if out.Len() != 0 {
		t.Errorf("wrote %q, want nothing", out.String())
	}
}

func TestRunLeavesTheOutputFileAloneWhenARuleFileDoesNotParse(t *testing.T) {
	rules := t.TempDir()
	writeFile(t, rules, "bad.yml", `
groups:
  - name: g
    rules:
      - alert: A
        expr: "broken{"
`)
	outDir := t.TempDir()
	writeFile(t, outDir, "generated.yml", "previous")
	output := filepath.Join(outDir, "generated.yml")

	if err := run([]string{"--rule-files", filepath.Join(rules, "*.yml"), "--output", output}, io.Discard); err == nil {
		t.Error("got no error, want one")
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "previous" {
		t.Errorf("output file holds %q (%v), want it untouched", got, err)
	}
	if entries, err := os.ReadDir(outDir); err != nil || len(entries) != 1 {
		t.Errorf("output directory holds %v (%v), want the output file alone", entries, err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
