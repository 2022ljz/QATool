#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export GOCACHE="${GOCACHE:-$ROOT/.gocache}"

go run ./cmd/validator check \
  --schema layer/01_schema.yaml \
  --rules layer/02_rule_library.yaml \
  --presets layer/03_presets.yaml \
  --check layer/04_checks_summer_night.yaml \
  --data-dir table_config \
  --workers 4
  --out summer_night_new
