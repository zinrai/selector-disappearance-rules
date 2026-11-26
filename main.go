package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/prometheus/common/model"
)

type config struct {
	ruleFiles   string
	output      string
	seenWithin  model.Duration
	absentFor   model.Duration
	showVersion bool
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "selector-disappearance-rules:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}
	if cfg.showVersion {
		printVersion()
		return nil
	}
	selectors, err := loadSelectors(cfg.ruleFiles)
	if err != nil {
		return err
	}
	write := func(w io.Writer) error {
		return writeRules(w, selectors, cfg.seenWithin, cfg.absentFor)
	}
	if cfg.output == "" {
		return write(out)
	}
	return replaceFile(cfg.output, write)
}

func parseArgs(args []string) (config, error) {
	cfg := config{seenWithin: model.Duration(defaultSeenWithin), absentFor: model.Duration(defaultAbsentFor)}
	fs := flag.NewFlagSet("selector-disappearance-rules", flag.ContinueOnError)
	fs.StringVar(&cfg.ruleFiles, "rule-files", "", "glob pattern of the rule files to read")
	fs.StringVar(&cfg.output, "output", "", "file to replace with the generated rules (default: standard output)")
	fs.Func("seen-within", "how far back a selector counts as having resolved on a target (default 2d)", durationFlag(&cfg.seenWithin))
	fs.Func("absent-for", "how long a selector must not have resolved on a target (default 1h)", durationFlag(&cfg.absentFor))
	fs.BoolVar(&cfg.showVersion, "version", false, "print version information and exit")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if cfg.showVersion {
		return cfg, nil
	}

	if cfg.ruleFiles == "" {
		return config{}, errors.New("specify --rule-files with a glob pattern of the rule files")
	}
	// Not ignored: an unquoted pattern is expanded by the shell, which leaves --rule-files with the
	// first file only and the rest here, and those would be left out without a word.
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("unexpected argument %q: quote the --rule-files pattern", fs.Arg(0))
	}
	// Not left to the ruler: the expression stays valid but can never return anything, so every
	// rule would load and then record nothing, forever.
	if cfg.seenWithin <= cfg.absentFor {
		return config{}, errors.New("--seen-within must be longer than --absent-for")
	}
	return cfg, nil
}

func durationFlag(dst *model.Duration) func(string) error {
	return func(s string) error {
		d, err := model.ParseDuration(s)
		*dst = d
		return err
	}
}
