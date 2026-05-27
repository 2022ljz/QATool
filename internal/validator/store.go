package validator

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

func loadYAML[T any](path string) (*T, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out T
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &out, nil
}

func LoadStore(ctx context.Context, schemaPath, dataDir string, schema *SchemaConfig, target TargetConfig) (*TableStore, error) {
	store := &TableStore{Tables: map[string]*Table{}}
	var mu sync.Mutex
	errCh := make(chan error, len(schema.Tables))
	var wg sync.WaitGroup
	rootTable := schema.Root.LogicalTable
	targetKey := target.Key
	if targetKey == "" {
		targetKey = schema.Root.DefaultTargetKey
	}
	for logical, tableSchema := range schema.Tables {
		logical, tableSchema := logical, tableSchema
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			default:
			}
			path := resolveCSVPath(schemaPath, dataDir, schema.Dataset.BaseDir, tableSchema.File)
			t, err := loadCSV(logical, path, tableSchema.PrimaryKey, rowFilter(logical, tableSchema, rootTable, targetKey, target.Value))
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			store.Tables[logical] = t
			mu.Unlock()
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}
	for name, ts := range schema.Tables {
		if ts.EnabledField != "" {
			store.BuildIndex(name, ts.EnabledField)
		}
		store.BuildIndex(name, "activity_id")
		for field := range ts.ForeignKeys {
			store.BuildIndex(name, field)
		}
	}
	return store, nil
}

func rowFilter(logical string, tableSchema TableSchema, rootTable, targetKey, targetValue string) func(Row) bool {
	if targetValue == "" {
		return nil
	}
	if logical == rootTable {
		return func(row Row) bool {
			return row[targetKey] == targetValue
		}
	}
	if _, ok := tableSchema.Fields["activity_id"]; ok {
		return func(row Row) bool {
			return row["activity_id"] == targetValue
		}
	}
	return nil
}

func resolveCSVPath(schemaPath, dataDir, baseDir, file string) string {
	if dataDir != "" {
		return filepath.Join(dataDir, file)
	}
	schemaDir := filepath.Dir(schemaPath)
	candidates := []string{
		filepath.Join(schemaDir, baseDir, file),
		filepath.Join("table_config", file),
		file,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0]
}

func loadCSV(logical, path, pk string, keep func(Row) bool) (*Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("load csv %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header %s: %w", path, err)
	}
	t := &Table{LogicalName: logical, FileName: path, PrimaryKey: pk, PKIndex: map[string]Row{}, Indexes: map[string]map[string][]Row{}}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv %s: %w", path, err)
		}
		row := Row{}
		for i, h := range headers {
			h = strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")
			if i < len(rec) {
				row[h] = strings.TrimSpace(rec[i])
			} else {
				row[h] = ""
			}
		}
		if keep != nil && !keep(row) {
			continue
		}
		t.Rows = append(t.Rows, row)
		t.PKIndex[row[pk]] = row
	}
	return t, nil
}

func (s *TableStore) GetTable(logicalName string) (*Table, bool) {
	t, ok := s.Tables[logicalName]
	return t, ok
}

func (s *TableStore) GetRow(table, key string) (Row, bool) {
	t, ok := s.GetTable(table)
	if !ok {
		return nil, false
	}
	r, ok := t.PKIndex[key]
	return r, ok
}

func (s *TableStore) FindRows(table, field, value string) []Row {
	t, ok := s.GetTable(table)
	if !ok {
		return nil
	}
	if idx, ok := t.Indexes[field]; ok {
		if rows := idx[value]; len(rows) > 0 {
			return append([]Row(nil), rows...)
		}
	}
	var rows []Row
	for _, r := range t.Rows {
		if r[field] == value {
			rows = append(rows, r)
		}
	}
	return rows
}

func (s *TableStore) BuildIndex(table, field string) {
	t, ok := s.GetTable(table)
	if !ok || field == "" {
		return
	}
	idx := map[string][]Row{}
	for _, r := range t.Rows {
		idx[r[field]] = append(idx[r[field]], r)
	}
	t.Indexes[field] = idx
}

func enabled(row Row, table TableSchema, checks DefaultChecksConfig) bool {
	if !checks.EnabledFilter || table.EnabledField == "" {
		return true
	}
	return parseBool(row[table.EnabledField])
}
