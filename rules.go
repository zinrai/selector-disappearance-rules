package main

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"go.yaml.in/yaml/v3"
)

type ruleFile struct {
	Groups []ruleGroup `yaml:"groups"`
}

type ruleGroup struct {
	Name  string     `yaml:"name"`
	Rules []ruleSpec `yaml:"rules"`
}

type ruleSpec struct {
	Alert string `yaml:"alert"`
	Expr  string `yaml:"expr"`
}

var exprParser = parser.NewParser(parser.Options{})

// loadSelectors fails on the first unreadable file or unparsable expr rather than returning what
// did parse: the output replaces an earlier one, so a partial result would silently drop the rules
// of every selector that went missing from it.
func loadSelectors(pattern string) ([]string, error) {
	paths, err := ruleFilePaths(pattern)
	if err != nil {
		return nil, err
	}
	unique := map[string]struct{}{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		selectors, err := extractSelectors(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		for _, sel := range selectors {
			unique[sel] = struct{}{}
		}
	}
	// Sorted, not in file order: regenerating from the same rules must give a byte-identical file.
	return slices.Sorted(maps.Keys(unique)), nil
}

func ruleFilePaths(pattern string) ([]string, error) {
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("--rule-files %q: %w", pattern, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("--rule-files %q matches no file", pattern)
	}
	return paths, nil
}

func extractSelectors(data []byte) ([]string, error) {
	var rf ruleFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, err
	}
	var out []string
	for _, g := range rf.Groups {
		selectors, err := groupSelectors(g)
		if err != nil {
			return nil, err
		}
		out = append(out, selectors...)
	}
	return out, nil
}

func groupSelectors(g ruleGroup) ([]string, error) {
	var out []string
	for _, r := range g.Rules {
		if r.Alert == "" {
			continue
		}
		selectors, err := selectorsOf(r.Expr)
		if err != nil {
			return nil, fmt.Errorf("group %q alert %q: %w", g.Name, r.Alert, err)
		}
		out = append(out, selectors...)
	}
	return out, nil
}

func selectorsOf(expr string) ([]string, error) {
	ast, err := exprParser.ParseExpr(expr)
	if err != nil {
		return nil, err
	}
	var selectors []string
	parser.Inspect(ast, func(node parser.Node, _ []parser.Node) error {
		if vs, ok := node.(*parser.VectorSelector); ok && !selectsOwnRecord(vs) {
			selectors = append(selectors, bare(vs))
		}
		return nil
	})
	return selectors, nil
}

// Not given a rule like any other selector: an alert that reads the record names it, and the
// record ending would then be recorded as a disappearance of its own, again each time that ended.
func selectsOwnRecord(vs *parser.VectorSelector) bool {
	for _, m := range vs.LabelMatchers {
		if m.Name == labels.MetricName && m.Matches(recordName) {
			return true
		}
	}
	return false
}

// Not vs.String(): that keeps the offset and @ modifiers of the alert expr, and the generated expr
// puts a range after the selector, which cannot follow a modifier.
func bare(vs *parser.VectorSelector) string {
	return (&parser.VectorSelector{Name: vs.Name, LabelMatchers: vs.LabelMatchers}).String()
}
