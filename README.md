# Game Config Validator

Game Config Validator is a local CLI tool for validating business-level consistency across game activity configuration tables.

It is designed for QA workflows before entering the game client. The tool does not replace export-table validation or runtime testing. It focuses on risks that can be detected from data, such as activity time windows, redpoints, signin rewards, activity currencies, exchange rewards, reward pools, and item references.

## Features

- Loads 10 CSV config tables from `table_config/`
- Loads the 4-layer YAML config model from `layer/`
- Checks one target activity by `target + preset`
- Supports schema defaults:
  - primary key uniqueness
  - foreign key existence
- Supports business checkers:
  - lifecycle time windows
  - cross-table field consistency
  - parameter equality
  - group uniqueness, count, max, aggregate compare
  - parent-child relationship checks
  - dependency cycle detection
- Generates a Markdown report
- Includes an integration test for `summer_night_2024`

## Quick Start

Run the default sample check:

```bash
bash scripts/check.sh
```

The script wraps the default paths:

```text
schema   -> layer/01_schema.yaml
rules    -> layer/02_rule_library.yaml
presets  -> layer/03_presets.yaml
check    -> layer/04_checks_summer_night.yaml
data     -> table_config/
```

The generated report is written to the path configured in the check file:

```text
reports/summer_night_2024.md
```

## Runner Fields

`scripts/check.sh` is the recommended entry point. It runs the validator with the project defaults:

| Field | Value | Description |
| --- | --- | --- |
| `GOCACHE` | `${GOCACHE:-$ROOT/.gocache}` | Go build cache. If the environment already has `GOCACHE`, the script keeps it; otherwise it uses the project-local `.gocache/`. |
| `--schema` | `layer/01_schema.yaml` | Table schema, primary keys, foreign keys, time fields, and CSV file mapping. |
| `--rules` | `layer/02_rule_library.yaml` | Rule template library. |
| `--presets` | `layer/03_presets.yaml` | Business preset definitions. |
| `--check` | `layer/04_checks_summer_night.yaml` | QA check entry file. |
| `--data-dir` | `table_config` | CSV config table directory. |
| `--workers` | `4` | Number of workers used by the rule engine. |

The script currently has no positional arguments. To change the target activity, preset, params, or output path, edit the layer-4 check file.

## CLI Fields

The underlying CLI still supports direct flags for development or debugging:

```bash
go run ./cmd/validator check \
  --schema layer/01_schema.yaml \
  --rules layer/02_rule_library.yaml \
  --presets layer/03_presets.yaml \
  --check layer/04_checks_summer_night.yaml \
  --data-dir table_config \
  --out reports/summer_night_2024.md \
  --workers 4
```

| Flag | Required | Default | Description |
| --- | --- | --- | --- |
| `--schema` | No | `layer/01_schema.yaml` | Schema YAML path. |
| `--rules` | No | `layer/02_rule_library.yaml` | Rule library YAML path. |
| `--presets` | No | `layer/03_presets.yaml` | Presets YAML path. |
| `--check`, `-c` | No | `layer/04_checks_summer_night.yaml` | Single-run check YAML path. |
| `--data-dir` | No | empty | CSV data directory. When empty, the loader tries schema-relative paths and then `table_config/`. |
| `--out` | No | empty | Report output path. Takes priority over `output.path` in the check YAML. |
| `--workers` | No | `4` | Rule execution worker count. |

## Check File Fields

The layer-4 check file controls normal QA input:

| Field | Description |
| --- | --- |
| `version` | Check config version. |
| `target.table` | Logical target table, usually `activity`. |
| `target.key` | Target key field, usually `activity_id`. |
| `target.value` | Target activity ID, for example `summer_night_2024`. |
| `target.name` | Optional display name for the activity. |
| `preset` | Preset name from `03_presets.yaml`, for example `activity_full_check`. |
| `params` | Business parameters used by preset rules, such as `currency_id`, `weekly_limit`, `expected_reset_weekday`, and `activity_weeks`. |
| `extra_checks` | Optional additional rule instances appended after preset expansion. |
| `skip` | Optional rule skip list by rule instance `id` or `rule`. |
| `output.format` | Report format. Current implementation writes Markdown. |
| `output.path` | Report output path used when `--out` is not provided. |
| `output.path_base` | Reserved path base hint from config. |
| `output.include_passed` | Reserved flag for including passed checks in reports. |

## Project Structure

```text
.
|-- cmd/validator/              # CLI entry
|-- internal/validator/          # Core validator implementation
|-- layer/                       # 4-layer YAML config examples
|-- scripts/
|   `-- check.sh                 # Default runner
|-- table_config/                # CSV config fixtures
|-- reports/                     # Generated reports, ignored by git
|-- go.mod
`-- go.sum
```

## 4-Layer Config Model

| Layer | File | Purpose |
| --- | --- | --- |
| 1 | `01_schema.yaml` | Declares tables, CSV files, primary keys, fields, and foreign keys |
| 2 | `02_rule_library.yaml` | Defines reusable rule templates and required parameters |
| 3 | `03_presets.yaml` | Assembles rule templates into business presets |
| 4 | `04_checks_summer_night.yaml` | QA entry file: target activity, preset, params, output |

Most QA usage should only touch layer 4.

## Example Target

```yaml
target:
  table: activity
  key: activity_id
  value: summer_night_2024

preset: activity_full_check
```

## Example Findings

The fixture data intentionally contains invalid records. The sample report can detect issues such as:

- redpoint time exceeding activity time
- redpoint open level lower than activity open level
- missing parent redpoint
- signin time exceeding activity time
- duplicated signin `day_no`
- signin `day_no` greater than `total_days`
- missing reward references
- currency weekly limit inconsistent with UI display limit
- task reward currency inconsistent with activity currency
- exchange cost currency inconsistent with activity currency
- exchange display reward inconsistent with actual reward
- reward pool broken links
- reward referencing a missing item

## Tests

```bash
go test ./...
```

If the default Go build cache has permission issues, the runner script already uses a project-local cache:

```bash
bash scripts/check.sh
```

## Design Boundary

This tool only validates configuration data relationships.

It does not validate:

- real UI rendering
- animation playback
- actual click behavior
- real reward delivery
- real currency deduction
- NPC, scene, or VFX runtime behavior

Those still require in-game testing.

## License

No license has been declared yet. Add one before publishing as an open-source repository.
