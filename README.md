# Game Config Validator

Game Config Validator is a local CLI tool for validating business-level consistency across game activity configuration tables.

It is designed for QA workflows before entering the game client. The tool does not replace export-table validation or runtime testing. It focuses on risks that can be detected from data, such as activity time windows, redpoints, signin rewards, activity currencies, exchange rewards, reward pools, and item references.

## Features

- Loads CSV config tables from `table_config/` with target-aware row filtering
- Loads the 4-layer YAML config model from `layer/`
- Checks one target activity by `target + preset`
- Keeps only target `activity_id` rows for activity-scoped tables during CSV reading
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

## Data Loading

The loader reads CSV files table by table. Different tables are loaded concurrently, while each CSV file is read sequentially in streaming style.

Rows are filtered before entering memory:

- The root `activity` table keeps only the target activity row.
- Tables with an `activity_id` field keep only rows matching `target.value`.
- Shared reference tables without `activity_id`, such as `reward` and `item`, are currently kept in full so indirect foreign-key checks can still resolve referenced rows.

After filtering, the remaining rows are stored in `TableStore` and indexed by primary key, `activity_id`, enabled field, and declared foreign-key fields.

This avoids loading every row from large activity-scoped tables for a single-activity QA run. A future optimization can further trim shared reference tables by first collecting referenced IDs from activity-scoped rows.

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
| `params` | Business parameters consumed by the selected preset and extra checks. Required preset params are declared in layer 3; extra checks may reference additional params with `param: name` or `$params.name`. Unused params are rejected. |
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
| 3 | `03_presets.yaml` | Assembles rule templates into business presets and declares required/optional preset params |
| 4 | `04_checks_summer_night.yaml` | QA entry file: target activity, preset, params, output |

Most QA usage should only touch layer 4.

## Example Target

```yaml
target:
  table: activity
  key: activity_id
  value: summer_night_2024

preset: activity_full_check

params:
  currency_id: NIGHT_JADE
  weekly_limit: 120
  activity_weeks: 2
```

For `activity_full_check`, layer 3 declares `currency_id`, `weekly_limit`, and `activity_weeks` as required params. The sample check also defines `expected_reset_weekday` because its `extra_checks` section references that param.

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
