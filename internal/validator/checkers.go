package validator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func scopedRows(c *CheckContext, table string) (*Table, []Row, error) {
	t, ok := c.Store.GetTable(table)
	if !ok {
		return nil, nil, fmt.Errorf("table %s not loaded", table)
	}
	ts := c.Schema.Tables[table]
	var rows []Row
	if table == c.Schema.Root.LogicalTable {
		if r, ok := c.Store.GetRow(table, c.Scope.ActivityID); ok && enabled(r, ts, c.Schema.DefaultChecks) {
			rows = append(rows, r)
		}
		return t, rows, nil
	}
	if _, has := firstRowField(t, "activity_id"); has {
		for _, r := range c.Store.FindRows(table, "activity_id", c.Scope.ActivityID) {
			if enabled(r, ts, c.Schema.DefaultChecks) {
				rows = append(rows, r)
			}
		}
		return t, rows, nil
	}
	for _, r := range t.Rows {
		if enabled(r, ts, c.Schema.DefaultChecks) {
			rows = append(rows, r)
		}
	}
	return t, rows, nil
}

func firstRowField(t *Table, field string) (string, bool) {
	if len(t.Rows) == 0 {
		return "", false
	}
	_, ok := t.Rows[0][field]
	return "", ok
}

func checkSameTableFieldMatch(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, fieldA, fieldB := s(rule, "table"), s(rule, "field_a"), s(rule, "field_b")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if r[fieldA] != r[fieldB] {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), fieldA+","+fieldB, r[fieldA], r[fieldB]))
		}
	}
	return issues, nil
}

func checkValueEqualsParam(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, field, param := s(rule, "table"), s(rule, "field"), s(rule, "param")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	expected := asString(c.Params[param])
	var issues []Issue
	for _, r := range rows {
		if r[field] != expected {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), field, r[field], expected))
		}
	}
	return issues, nil
}

func checkValueInSet(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, field := s(rule, "table"), s(rule, "field")
	allowed := map[string]bool{}
	for _, v := range asStringSlice(rule.With["allowed"]) {
		allowed[v] = true
	}
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if !allowed[r[field]] {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), field, r[field], strings.Join(asStringSlice(rule.With["allowed"]), "|")))
		}
	}
	return issues, nil
}

func checkFieldMatchViaFK(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	return checkViaFK(c, rule, "==")
}

func checkFieldCompareViaFK(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	return checkViaFK(c, rule, s(rule, "operator"))
}

func checkRefreshWeekdayMatch(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	return checkViaFK(c, rule, "==")
}

func checkViaFK(c *CheckContext, rule RuleInstance, op string) ([]Issue, error) {
	child, parent := s(rule, "child_table"), s(rule, "parent_table")
	fk, childField, parentField := s(rule, "fk_field"), s(rule, "child_field"), s(rule, "parent_field")
	ct, rows, err := scopedRows(c, child)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if rule.Rule == "refresh_weekday_match" && strings.TrimSpace(r[childField]) == "" {
			continue
		}
		pr, ok := c.Store.GetRow(parent, r[fk])
		if !ok {
			continue
		}
		okCmp, err := compare(r[childField], pr[parentField], op)
		if err != nil || !okCmp {
			issues = append(issues, newIssue(rule, child, rowKey(ct, r), childField, r[childField], op+" "+pr[parentField]))
		}
	}
	return issues, nil
}

func checkStartEndValid(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table := s(rule, "table")
	start, end := sDefault(rule, "start_field", c.Schema.Tables[table].TimeFields.Start), sDefault(rule, "end_field", c.Schema.Tables[table].TimeFields.End)
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		st, se := parseTimeValue(r[start])
		et, ee := parseTimeValue(r[end])
		if se != nil || ee != nil || !st.Before(et) {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), start+","+end, r[start]+".."+r[end], "start < end"))
		}
	}
	return issues, nil
}

func checkTimeWindowWithin(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	child, parent, fk := s(rule, "child_table"), s(rule, "parent_table"), s(rule, "fk_field")
	cs := c.Schema.Tables[child].TimeFields
	ps := c.Schema.Tables[parent].TimeFields
	childStart, childEnd := sDefault(rule, "child_start", cs.Start), sDefault(rule, "child_end", cs.End)
	parentStart, parentEnd := sDefault(rule, "parent_start", ps.Start), sDefault(rule, "parent_end", ps.End)
	ct, rows, err := scopedRows(c, child)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		pr, ok := c.Store.GetRow(parent, r[fk])
		if !ok {
			continue
		}
		cst, e1 := parseTimeValue(r[childStart])
		cet, e2 := parseTimeValue(r[childEnd])
		pst, e3 := parseTimeValue(pr[parentStart])
		pet, e4 := parseTimeValue(pr[parentEnd])
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || cst.Before(pst) || cet.After(pet) {
			issues = append(issues, newIssue(rule, child, rowKey(ct, r), childStart+","+childEnd, r[childStart]+".."+r[childEnd], pr[parentStart]+".."+pr[parentEnd]))
		}
	}
	return issues, nil
}

func checkFieldUniqueInGroup(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, groupBy, field := s(rule, "table"), s(rule, "group_by"), s(rule, "unique_field")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	seen := map[string]Row{}
	var issues []Issue
	for _, r := range rows {
		key := r[groupBy] + "\x00" + r[field]
		if first, ok := seen[key]; ok {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), field, r[field], "unique in "+groupBy+" (first "+rowKey(t, first)+")"))
		} else {
			seen[key] = r
		}
	}
	return issues, nil
}

func checkCountEqualsField(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	child, groupBy, parent := s(rule, "child_table"), s(rule, "group_by"), s(rule, "parent_table")
	parentKey, parentField := s(rule, "parent_key"), s(rule, "parent_field")
	_, rows, err := scopedRows(c, child)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, r := range rows {
		counts[r[groupBy]]++
	}
	pt, parents, err := scopedRows(c, parent)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, p := range parents {
		actual := strconv.Itoa(counts[p[parentKey]])
		if actual != p[parentField] {
			issues = append(issues, newIssue(rule, parent, rowKey(pt, p), parentField, actual, p[parentField]))
		}
	}
	return issues, nil
}

func checkMaxFieldLTEParentField(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	child, groupBy, field := s(rule, "child_table"), s(rule, "group_by"), s(rule, "field")
	parent, parentKey, parentField := s(rule, "parent_table"), s(rule, "parent_key"), s(rule, "parent_field")
	_, rows, err := scopedRows(c, child)
	if err != nil {
		return nil, err
	}
	maxes := map[string]float64{}
	for _, r := range rows {
		v, _ := strconv.ParseFloat(r[field], 64)
		if v > maxes[r[groupBy]] {
			maxes[r[groupBy]] = v
		}
	}
	pt, parents, err := scopedRows(c, parent)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, p := range parents {
		limit, _ := strconv.ParseFloat(p[parentField], 64)
		if maxes[p[parentKey]] > limit {
			issues = append(issues, newIssue(rule, child, p[parentKey], field, fmt.Sprintf("%.0f", maxes[p[parentKey]]), "<= "+p[parentField]))
		}
		_ = pt
	}
	return issues, nil
}

func checkAggregateCompare(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	left := aggregate(c, s(rule, "left_table"), s(rule, "left_field"), s(rule, "left_agg"), mapFromAny(rule.With["left_where"]))
	right := aggregate(c, s(rule, "right_table"), s(rule, "right_field"), s(rule, "right_agg"), mapFromAny(rule.With["right_where"]))
	mul := 1.0
	if v := asString(rule.With["multiplier"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			mul = f
		}
	}
	left *= mul
	if !compareFloat(left, right, s(rule, "operator")) {
		return []Issue{newIssue(rule, s(rule, "left_table")+"+"+s(rule, "right_table"), c.Scope.ActivityID, s(rule, "left_field"), fmt.Sprintf("%.2f", left), s(rule, "operator")+" "+fmt.Sprintf("%.2f", right))}, nil
	}
	return nil, nil
}

func aggregate(c *CheckContext, table, field, agg string, where map[string]string) float64 {
	_, rows, err := scopedRows(c, table)
	if err != nil {
		return 0
	}
	count, sum, max, min := 0.0, 0.0, 0.0, 0.0
	first := true
	for _, r := range rows {
		if !matchesWhere(r, where) {
			continue
		}
		v, _ := strconv.ParseFloat(r[field], 64)
		count++
		sum += v
		if first || v > max {
			max = v
		}
		if first || v < min {
			min = v
		}
		first = false
	}
	switch agg {
	case "count":
		return count
	case "max":
		return max
	case "min":
		return min
	default:
		return sum
	}
}

func checkParentExists(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, parentKey := s(rule, "table"), s(rule, "parent_key")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if r[parentKey] == "" {
			continue
		}
		if _, ok := c.Store.GetRow(table, r[parentKey]); !ok {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), parentKey, r[parentKey], "existing parent"))
		}
	}
	return issues, nil
}

func checkParentChildFieldMatch(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, parentKey, field := s(rule, "table"), s(rule, "parent_key"), s(rule, "field")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if r[parentKey] == "" {
			continue
		}
		p, ok := c.Store.GetRow(table, r[parentKey])
		if ok && r[field] != p[field] {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), field, r[field], p[field]))
		}
	}
	return issues, nil
}

func checkParentChildTimeWithin(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, parentKey := s(rule, "table"), s(rule, "parent_key")
	start, end := sDefault(rule, "start_field", c.Schema.Tables[table].TimeFields.Start), sDefault(rule, "end_field", c.Schema.Tables[table].TimeFields.End)
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		if r[parentKey] == "" {
			continue
		}
		p, ok := c.Store.GetRow(table, r[parentKey])
		if !ok {
			continue
		}
		cs, _ := parseTimeValue(r[start])
		ce, _ := parseTimeValue(r[end])
		ps, _ := parseTimeValue(p[start])
		pe, _ := parseTimeValue(p[end])
		if cs.Before(ps) || ce.After(pe) {
			issues = append(issues, newIssue(rule, table, rowKey(t, r), start+","+end, r[start]+".."+r[end], p[start]+".."+p[end]))
		}
	}
	return issues, nil
}

func checkNoCycle(_ context.Context, c *CheckContext, rule RuleInstance) ([]Issue, error) {
	table, selfKey, parentKey := s(rule, "table"), s(rule, "self_key"), s(rule, "parent_key")
	t, rows, err := scopedRows(c, table)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	for _, r := range rows {
		seen := map[string]bool{}
		cur := r
		for cur[parentKey] != "" {
			next := cur[parentKey]
			if seen[next] || next == r[selfKey] {
				issues = append(issues, newIssue(rule, table, rowKey(t, r), parentKey, r[parentKey], "acyclic chain"))
				break
			}
			seen[next] = true
			nr, ok := c.Store.GetRow(table, next)
			if !ok {
				break
			}
			cur = nr
		}
	}
	return issues, nil
}

func s(rule RuleInstance, key string) string { return asString(rule.With[key]) }
func sDefault(rule RuleInstance, key, def string) string {
	if v := s(rule, key); v != "" {
		return v
	}
	return def
}

func mapFromAny(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range m {
		out[k] = asString(v)
	}
	return out
}

func matchesWhere(row Row, where map[string]string) bool {
	for k, v := range where {
		if row[k] != v {
			return false
		}
	}
	return true
}
