package main

import (
	"fmt"
	"io"
	"time"

	"github.com/prometheus/common/model"
	"go.yaml.in/yaml/v3"
)

const (
	groupName     = "selector-disappearance"
	recordName    = "selector_disappeared_series"
	selectorLabel = "selector"

	defaultSeenWithin = 48 * time.Hour
	defaultAbsentFor  = time.Hour
)

type generatedFile struct {
	Groups []generatedGroup `yaml:"groups"`
}

type generatedGroup struct {
	Name  string          `yaml:"name"`
	Rules []generatedRule `yaml:"rules"`
}

type generatedRule struct {
	Record string            `yaml:"record"`
	Expr   string            `yaml:"expr"`
	Labels map[string]string `yaml:"labels"`
}

func writeRules(out io.Writer, selectors []string, seenWithin, absentFor model.Duration) error {
	group := generatedGroup{Name: groupName, Rules: make([]generatedRule, len(selectors))}
	for i, sel := range selectors {
		group.Rules[i] = generatedRule{
			Record: recordName,
			Expr:   disappearedExpr(sel, seenWithin, absentFor),
			Labels: map[string]string{selectorLabel: sel},
		}
	}
	enc := yaml.NewEncoder(out)
	enc.SetIndent(2)
	if err := enc.Encode(generatedFile{Groups: []generatedGroup{group}}); err != nil {
		return err
	}
	return enc.Close()
}

// Ranges on both sides, not instants: an instant on the right records every target that merely
// skipped a scrape, and one on the left misses a target that had skipped one at that moment.
// No `up == 1` filter here: a filter baked in could never be taken back out of the recorded history.
func disappearedExpr(selector string, seenWithin, absentFor model.Duration) string {
	return fmt.Sprintf(
		"count by (job, instance) (present_over_time(%s[%s])) unless count by (job, instance) (present_over_time(%s[%s]))",
		selector, seenWithin, selector, absentFor)
}
