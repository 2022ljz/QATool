package validator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummerNightIntegration(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.md")
	_, err := Run(context.Background(), Options{
		SchemaPath:  filepath.FromSlash("../../layer/01_schema.yaml"),
		RulesPath:   filepath.FromSlash("../../layer/02_rule_library.yaml"),
		PresetsPath: filepath.FromSlash("../../layer/03_presets.yaml"),
		CheckPath:   filepath.FromSlash("../../layer/04_checks_summer_night.yaml"),
		DataDir:     filepath.FromSlash("../../table_config"),
		OutPath:     out,
		Workers:     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	report := string(b)
	for _, want := range []string{
		"redpoint.RP_SUMMER_REWARD",
		"signin.SIG_SUMMER_NIGHT",
		"signin_reward.SR_SUMMER_DUP_DAY3",
		"currency.NIGHT_JADE",
		"task.TASK_SUMMER_BAD_CURRENCY",
		"exchange.EX_SUMMER_CORE_1",
		"reward_pool.POOL_SUMMER_STAGE2",
		"reward.RWD_POOL_STAGE2_ITEM",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %s\n%s", want, report)
		}
	}
	if strings.Contains(report, "task.TASK_SUMMER_DAILY_1") {
		t.Fatalf("daily task without refresh_weekday should not fail refresh weekday rule")
	}
}

func TestCompare(t *testing.T) {
	ok, err := compare("30", "20", ">=")
	if err != nil || !ok {
		t.Fatalf("numeric compare failed: ok=%v err=%v", ok, err)
	}
	ok, err = compare("NIGHT_JADE", "MOON_TOKEN", "==")
	if err != nil || ok {
		t.Fatalf("string compare failed: ok=%v err=%v", ok, err)
	}
}
