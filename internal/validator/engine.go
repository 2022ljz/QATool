package validator

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Checker interface {
	Check(context.Context, *CheckContext, RuleInstance) ([]Issue, error)
}

type checkerFunc func(context.Context, *CheckContext, RuleInstance) ([]Issue, error)

func (f checkerFunc) Check(ctx context.Context, c *CheckContext, r RuleInstance) ([]Issue, error) {
	return f(ctx, c, r)
}

type Registry struct {
	checkers map[string]Checker
}

func NewRegistry() *Registry {
	r := &Registry{checkers: map[string]Checker{}}
	r.Register("same_table_field_match", checkerFunc(checkSameTableFieldMatch))
	r.Register("value_equals_param", checkerFunc(checkValueEqualsParam))
	r.Register("value_in_set", checkerFunc(checkValueInSet))
	r.Register("field_match_via_fk", checkerFunc(checkFieldMatchViaFK))
	r.Register("field_compare_via_fk", checkerFunc(checkFieldCompareViaFK))
	r.Register("time_window_within", checkerFunc(checkTimeWindowWithin))
	r.Register("start_end_valid", checkerFunc(checkStartEndValid))
	r.Register("refresh_weekday_match", checkerFunc(checkRefreshWeekdayMatch))
	r.Register("field_unique_in_group", checkerFunc(checkFieldUniqueInGroup))
	r.Register("count_equals_field", checkerFunc(checkCountEqualsField))
	r.Register("max_field_lte_parent_field", checkerFunc(checkMaxFieldLTEParentField))
	r.Register("aggregate_compare", checkerFunc(checkAggregateCompare))
	r.Register("parent_exists", checkerFunc(checkParentExists))
	r.Register("parent_child_field_match", checkerFunc(checkParentChildFieldMatch))
	r.Register("parent_child_time_within", checkerFunc(checkParentChildTimeWithin))
	r.Register("no_cycle", checkerFunc(checkNoCycle))
	return r
}

func (r *Registry) Register(ruleID string, c Checker) { r.checkers[ruleID] = c }
func (r *Registry) Get(ruleID string) (Checker, bool) { c, ok := r.checkers[ruleID]; return c, ok }

type Engine struct {
	Registry *Registry
	Workers  int
}

func (e *Engine) Run(ctx context.Context, c *CheckContext, rules []RuleInstance) ([]Issue, error) {
	workers := e.Workers
	if workers <= 0 {
		workers = 4
	}
	jobs := make(chan RuleInstance)
	out := make(chan []Issue, len(rules))
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rule := range jobs {
				checker, ok := e.Registry.Get(rule.Rule)
				if !ok {
					select {
					case errCh <- fmt.Errorf("rule %s is not registered", rule.Rule):
					default:
					}
					continue
				}
				issues, err := checker.Check(ctx, c, rule)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}
				out <- issues
			}
		}()
	}
	for _, r := range rules {
		select {
		case jobs <- r:
		case err := <-errCh:
			close(jobs)
			wg.Wait()
			return nil, err
		}
	}
	close(jobs)
	wg.Wait()
	close(out)
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	var issues []Issue
	for batch := range out {
		issues = append(issues, batch...)
	}
	sortIssues(issues)
	return issues, nil
}

func sortIssues(issues []Issue) {
	sev := map[string]int{"ERROR": 0, "WARN": 1, "INFO": 2}
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if sev[a.Severity] != sev[b.Severity] {
			return sev[a.Severity] < sev[b.Severity]
		}
		if a.Group != b.Group {
			return a.Group < b.Group
		}
		if a.Table != b.Table {
			return a.Table < b.Table
		}
		if a.RowKey != b.RowKey {
			return a.RowKey < b.RowKey
		}
		return a.RuleID < b.RuleID
	})
}

func newIssue(rule RuleInstance, table, key, field, actual, expected string) Issue {
	sev := rule.Severity
	if sev == "" {
		sev = "ERROR"
	}
	id := rule.ID
	if id == "" {
		id = rule.Rule
	}
	msg := rule.Message
	if msg == "" {
		msg = fmt.Sprintf("%s failed", rule.Rule)
	}
	return Issue{Severity: sev, RuleID: id, RuleName: rule.Rule, Group: rule.Group, Table: table, RowKey: key, Field: field, ActualValue: actual, ExpectedValue: expected, Message: msg, Suggestion: rule.Suggestion}
}
