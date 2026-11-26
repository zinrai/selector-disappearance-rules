# selector-disappearance-rules

Generates Prometheus recording rules that record, for every metric selector used in alert rules, the targets on which that selector resolved before and no longer does.

## Why this exists

An alert expression that returns no series does not fire and does not error. When a selector stops resolving on only some targets, the alert keeps working for the rest and goes quiet for exactly those targets. A check that asks whether a selector resolves anywhere stays green throughout.

PromQL can express the per-target comparison, but only one selector at a time and by hand. This tool writes that comparison for every selector the alert rules already use. It records where a selector stopped resolving. Why it stopped, and whether that matters, is for whoever is notified.

## Usage

Pass the alert rule files as a glob pattern with `--rule-files`, quoted so that the tool expands it rather than the shell, and the file to write with `--output`.

```
selector-disappearance-rules --rule-files 'rules/*.yml' --output generated/selector-disappearance.yml
```

The file is replaced only once the whole rule file has been generated, so a failed run leaves the previous one as it was. Without `--output` the rules go to standard output. A shell redirect empties its target before the tool starts, and a ruler takes an empty rule file as no rules rather than as an error.

For each unique selector the output holds one rule:

```yaml
groups:
  - name: selector-disappearance
    rules:
      - record: selector_disappeared_series
        expr: count by (job, instance) (present_over_time(http_requests_total{status=~"5.."}[2d])) unless count by (job, instance) (present_over_time(http_requests_total{status=~"5.."}[1h]))
        labels:
          selector: http_requests_total{status=~"5.."}
```

`selector_disappeared_series{selector, job, instance}` exists only while a target resolved the selector within `--seen-within` and has not resolved it for `--absent-for`. Its value is the number of series the target had within `--seen-within`. Nothing is recorded for a target that still resolves the selector.

Reading the result is left to the consumer:

```promql
# Targets that lost a selector while still being scraped.
count by (job, instance) (selector_disappeared_series and on (job, instance) up == 1)
```

One cause usually takes several selectors away from a target at once, so counting by target gives one result for it rather than one per selector.

The record is there to get a disappearance noticed, not to follow it afterwards. It ends when the selector resolves again and also when `--seen-within` has passed, so its end does not mean the selector is back.

## Options

- `--rule-files` is a glob pattern of the alert rule files to read.
- `--output` is the file to replace with the generated rules. Default: standard output.
- `--seen-within` is how far back a selector counts as having resolved on a target. Default `2d`. A disappearance is recorded until this long has passed since the selector last resolved, and then drops out. It has to outlast `--absent-for`, the longest outage of rule evaluation or alerting that should not swallow a disappearance, and the time the alert reading the record then needs to fire. The default allows for an outage of about a day.
- `--absent-for` is how long a selector must not have resolved on a target before it is recorded. Default `1h`. A target that only skips a scrape now and then is not recorded.

## Behaviour

- Only `alert:` rules are read. `record:` rules are ignored, so the generated file can sit next to the rules it was generated from.
- A selector that selects `selector_disappeared_series` itself gets no rule, so an alert that reads the record can live among the alert rules.
- A selector keeps its label matchers and loses its `offset` and `@` modifiers. Selectors are deduplicated across all files and sorted, so the same rules always give the same output.
- A rule file that cannot be read or an expression that cannot be parsed fails the run and nothing is written. The output replaces an earlier one, so writing only what parsed would silently drop rules.
- Targets are identified by `job` and `instance`. Series that carry neither collapse into one result with no target labels, which then reports the selector disappearing as a whole.
- Whether a target is down or was removed is not judged. The recorded series keeps `job` and `instance`, so that is a join against `up` on the consumer side, as above.

## Limitations

- A target on which a selector never resolved is not recorded. That it resolved before is the only reason to expect it.
- A target that keeps some series of a selector and loses others is not recorded.
- A selector that is legitimately absent from a target for longer than `--absent-for` is recorded.
- Every rule reads `--seen-within` of history for its selector across all targets. Evaluation cost grows with the series a selector matches and with `--seen-within`, not with what is recorded. One evaluation of the whole group has to fit in the ruler's evaluation interval.
- A reader sees the record only for as long as its server treats the last sample as current. With an evaluation interval longer than that, read it as `last_over_time(selector_disappeared_series[10m])` with a range of at least the interval.
- Expressions are parsed with the Prometheus PromQL parser. Syntax specific to other query languages fails the run.

## License

This project is licensed under the [MIT License](LICENSE).
