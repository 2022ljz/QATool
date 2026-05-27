package validator

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const timeLayout = "2006-01-02 15:04:05"

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "y":
		return true
	default:
		return false
	}
}

func asString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if math.Trunc(x) == x {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func asStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			out = append(out, asString(item))
		}
		return out
	default:
		return nil
	}
}

func resolveValue(v any, params map[string]any, target TargetConfig) any {
	switch x := v.(type) {
	case string:
		if strings.HasPrefix(x, "$params.") {
			return params[strings.TrimPrefix(x, "$params.")]
		}
		if strings.HasPrefix(x, "$target.") {
			switch strings.TrimPrefix(x, "$target.") {
			case "table":
				return target.Table
			case "key":
				return target.Key
			case "value":
				return target.Value
			case "name":
				return target.Name
			}
		}
		return x
	case map[string]any:
		m := map[string]any{}
		for k, v := range x {
			m[k] = resolveValue(v, params, target)
		}
		return m
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = resolveValue(x[i], params, target)
		}
		return out
	default:
		return v
	}
}

func compare(left, right, op string) (bool, error) {
	lf, le := strconv.ParseFloat(left, 64)
	rf, re := strconv.ParseFloat(right, 64)
	if le == nil && re == nil {
		return compareFloat(lf, rf, op), nil
	}
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	default:
		return false, fmt.Errorf("non numeric compare %q %s %q", left, op, right)
	}
}

func compareFloat(left, right float64, op string) bool {
	switch op {
	case "==":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func parseTimeValue(v string) (time.Time, error) {
	return time.ParseInLocation(timeLayout, v, time.Local)
}

func rowKey(t *Table, row Row) string {
	if t == nil {
		return ""
	}
	if v := row[t.PrimaryKey]; v != "" {
		return v
	}
	if v := row["id"]; v != "" {
		return v
	}
	for _, key := range []string{"activity_id", "redpoint_id", "signin_id", "signin_reward_id", "currency_id", "task_id", "exchange_id", "pool_id", "reward_id", "item_id"} {
		if v := row[key]; v != "" {
			return v
		}
	}
	return ""
}
