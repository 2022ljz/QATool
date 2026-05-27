package validator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Run(ctx context.Context, opts Options) (string, error) {
	schema, err := loadYAML[SchemaConfig](opts.SchemaPath)
	if err != nil {
		return "", err
	}
	rules, err := loadYAML[RuleLibraryConfig](opts.RulesPath)
	if err != nil {
		return "", err
	}
	presets, err := loadYAML[PresetsConfig](opts.PresetsPath)
	if err != nil {
		return "", err
	}
	checks, err := loadYAML[ChecksConfig](opts.CheckPath)
	if err != nil {
		return "", err
	}
	if checks.Target.Table == "" {
		checks.Target.Table = schema.Root.LogicalTable
	}
	if checks.Target.Key == "" {
		checks.Target.Key = schema.Root.DefaultTargetKey
	}
	store, err := LoadStore(ctx, opts.SchemaPath, opts.DataDir, schema, checks.Target)
	if err != nil {
		return "", err
	}
	preset, ok := presets.Presets[checks.Preset]
	if !ok {
		return "", fmt.Errorf("preset %s not found", checks.Preset)
	}
	scope := Scope{TargetTable: checks.Target.Table, TargetKey: checks.Target.Key, TargetValue: checks.Target.Value, ActivityID: checks.Target.Value}
	if !targetExists(store, checks.Target) {
		return "", fmt.Errorf("target %s.%s=%q not found (%s)", checks.Target.Table, checks.Target.Key, checks.Target.Value, storeSummary(store))
	}
	cc := &CheckContext{Schema: schema, Store: store, Target: checks.Target, Params: checks.Params, Scope: scope}

	var issues []Issue
	if preset.IncludeSchemaDefaultChecks {
		issues = append(issues, runSchemaDefaults(cc)...)
	}
	instances, err := prepareRules(preset, *checks, *rules)
	if err != nil {
		return "", err
	}
	engine := &Engine{Registry: NewRegistry(), Workers: opts.Workers}
	ruleIssues, err := engine.Run(ctx, cc, instances)
	if err != nil {
		return "", err
	}
	issues = append(issues, ruleIssues...)
	sortIssues(issues)

	report := Report{Target: checks.Target, Preset: checks.Preset, Issues: issues, RuleCount: len(instances)}
	for _, issue := range issues {
		switch issue.Severity {
		case "ERROR":
			report.Summary.ErrorCount++
		case "WARN":
			report.Summary.WarnCount++
		}
	}
	failedRules := map[string]bool{}
	for _, issue := range issues {
		failedRules[issue.RuleID] = true
	}
	report.Summary.PassCount = report.RuleCount - len(failedRules)
	if report.Summary.PassCount < 0 {
		report.Summary.PassCount = 0
	}
	out := opts.OutPath
	if out == "" {
		out = checks.Output.Path
	}
	if out == "" {
		out = filepath.Join("reports", checks.Target.Value+".md")
	}
	if err := writeMarkdown(report, out); err != nil {
		return "", err
	}
	return out, nil
}

func targetExists(store *TableStore, target TargetConfig) bool {
	if target.Key == "" {
		_, ok := store.GetRow(target.Table, target.Value)
		return ok
	}
	rows := store.FindRows(target.Table, target.Key, target.Value)
	return len(rows) > 0
}

func storeSummary(store *TableStore) string {
	parts := make([]string, 0, len(store.Tables))
	for name, table := range store.Tables {
		sample := ""
		if len(table.Rows) > 0 {
			sample = table.Rows[0][table.PrimaryKey]
			if sample == "" {
				sample = table.Rows[0]["activity_id"]
			}
		}
		parts = append(parts, fmt.Sprintf("%s:%d pk=%q sample=%q", name, len(table.Rows), table.PrimaryKey, sample))
	}
	return strings.Join(parts, ", ")
}

func prepareRules(preset PresetConfig, checks ChecksConfig, lib RuleLibraryConfig) ([]RuleInstance, error) {
	var rules []RuleInstance
	rules = append(rules, preset.Templates...)
	rules = append(rules, preset.Checks...)
	rules = append(rules, checks.ExtraChecks...)
	for i, r := range rules {
		if r.ID == "" {
			r.ID = fmt.Sprintf("%s_%02d", r.Rule, i+1)
		}
		rules[i] = r
	}
	if err := validatePresetParams(preset, checks, rules); err != nil {
		return nil, err
	}
	filtered := rules[:0]
	for _, r := range rules {
		if skipped(r, checks.Skip) {
			continue
		}
		for k, v := range r.With {
			r.With[k] = resolveValue(v, checks.Params, checks.Target)
		}
		if err := validateRequired(r, lib); err != nil {
			return nil, err
		}
		filtered = append(filtered, r)
	}
	return filtered, nil
}

func validatePresetParams(preset PresetConfig, checks ChecksConfig, rules []RuleInstance) error {
	allowed := map[string]bool{}
	used := map[string]bool{}
	for _, k := range preset.RequiredParams {
		allowed[k] = true
		if _, ok := checks.Params[k]; !ok {
			return fmt.Errorf("preset %q missing required param %q", checks.Preset, k)
		}
	}
	for _, k := range preset.OptionalParams {
		allowed[k] = true
	}
	for _, r := range rules {
		if skipped(r, checks.Skip) {
			continue
		}
		for _, k := range referencedParams(r) {
			used[k] = true
			allowed[k] = true
			if _, ok := checks.Params[k]; !ok {
				return fmt.Errorf("rule %s references missing param %q", r.Rule, k)
			}
		}
	}
	for k := range checks.Params {
		if !allowed[k] && !used[k] {
			return fmt.Errorf("param %q is not declared by preset %q and is not referenced by any rule", k, checks.Preset)
		}
	}
	return nil
}

func referencedParams(rule RuleInstance) []string {
	seen := map[string]bool{}
	var out []string
	if v, ok := rule.With["param"]; ok {
		name := asString(v)
		if name != "" {
			seen[name] = true
			out = append(out, name)
		}
	}
	collectParamRefs(rule.With, seen, &out)
	return out
}

func collectParamRefs(v any, seen map[string]bool, out *[]string) {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "$params.") {
			name := strings.TrimPrefix(x, "$params.")
			if name != "" && !seen[name] {
				seen[name] = true
				*out = append(*out, name)
			}
		}
	case map[string]any:
		for _, item := range x {
			collectParamRefs(item, seen, out)
		}
	case []any:
		for _, item := range x {
			collectParamRefs(item, seen, out)
		}
	}
}

func skipped(rule RuleInstance, skips []SkipRule) bool {
	for _, s := range skips {
		if s.ID != "" && s.ID == rule.ID {
			return true
		}
		if s.Rule != "" && s.Rule == rule.Rule {
			return true
		}
	}
	return false
}

func validateRequired(rule RuleInstance, lib RuleLibraryConfig) error {
	for _, group := range lib.RuleGroups {
		for _, def := range group.Rules {
			if def.ID != rule.Rule {
				continue
			}
			for _, k := range def.Params.Required {
				if _, ok := rule.With[k]; !ok {
					return fmt.Errorf("rule %s missing required param %s", rule.Rule, k)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("rule %s is not defined in rule library", rule.Rule)
}

func runSchemaDefaults(c *CheckContext) []Issue {
	var issues []Issue
	if c.Schema.DefaultChecks.PrimaryKeyUnique {
		for name, t := range c.Store.Tables {
			seen := map[string]string{}
			for _, r := range t.Rows {
				key := r[t.PrimaryKey]
				if seen[key] != "" {
					rule := RuleInstance{ID: "schema_primary_key_unique", Rule: "primary_key_unique", Group: "schema", Severity: "ERROR", Message: "primary key must be unique"}
					issues = append(issues, newIssue(rule, name, key, t.PrimaryKey, key, "unique primary key"))
				}
				seen[key] = key
			}
		}
	}
	if c.Schema.DefaultChecks.ForeignKeyExists {
		for tableName, ts := range c.Schema.Tables {
			t, rows, err := scopedRows(c, tableName)
			if err != nil {
				continue
			}
			for _, row := range rows {
				for field, ref := range ts.ForeignKeys {
					val := row[field]
					if val == "" && c.Schema.DefaultChecks.IgnoreEmptyForeignKey {
						continue
					}
					refTable := strings.Split(ref, ".")[0]
					if _, ok := c.Store.GetRow(refTable, val); !ok {
						rule := RuleInstance{ID: "schema_foreign_key_exists", Rule: "foreign_key_exists", Group: "schema", Severity: "ERROR", Message: "foreign key reference must exist"}
						issues = append(issues, newIssue(rule, tableName, rowKey(t, row), field, val, ref))
					}
				}
			}
		}
	}
	return issues
}

func writeMarkdown(report Report, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Game Config Validator Report\n\n")
	b.WriteString("## Target\n\n")
	b.WriteString(fmt.Sprintf("- activity_id: `%s`\n- preset: `%s`\n\n", report.Target.Value, report.Preset))
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- ERROR: %d\n- WARN: %d\n- PASS: %d\n\n", report.Summary.ErrorCount, report.Summary.WarnCount, report.Summary.PassCount))
	b.WriteString("## Issues\n\n")
	if len(report.Issues) == 0 {
		b.WriteString("No issues found.\n")
		return os.WriteFile(path, []byte(b.String()), 0644)
	}
	for i, issue := range report.Issues {
		b.WriteString(fmt.Sprintf("### %d. [%s] %s `%s.%s`\n\n", i+1, issue.Severity, issue.Message, issue.Table, issue.RowKey))
		b.WriteString(fmt.Sprintf("- Rule: `%s` (`%s`)\n", issue.RuleID, issue.RuleName))
		b.WriteString(fmt.Sprintf("- Group: `%s`\n", issue.Group))
		b.WriteString(fmt.Sprintf("- Field: `%s`\n", issue.Field))
		b.WriteString(fmt.Sprintf("- Actual: `%s`\n", issue.ActualValue))
		b.WriteString(fmt.Sprintf("- Expected: `%s`\n", issue.ExpectedValue))
		if issue.Suggestion != "" {
			b.WriteString(fmt.Sprintf("- Suggestion: %s\n", issue.Suggestion))
		}
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
