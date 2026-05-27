package validator

type Options struct {
	SchemaPath  string
	RulesPath   string
	PresetsPath string
	CheckPath   string
	DataDir     string
	OutPath     string
	Workers     int
}

type SchemaConfig struct {
	Version       string                 `yaml:"version"`
	Root          RootConfig             `yaml:"root"`
	Dataset       DatasetConfig          `yaml:"dataset"`
	DefaultChecks DefaultChecksConfig    `yaml:"default_checks"`
	Tables        map[string]TableSchema `yaml:"tables"`
}

type RootConfig struct {
	LogicalTable     string `yaml:"logical_table"`
	PhysicalTable    string `yaml:"physical_table"`
	PrimaryKey       string `yaml:"primary_key"`
	DefaultTargetKey string `yaml:"default_target_key"`
}

type DatasetConfig struct {
	BaseDir string `yaml:"base_dir"`
	Format  string `yaml:"format"`
}

type DefaultChecksConfig struct {
	PrimaryKeyUnique      bool `yaml:"primary_key_unique"`
	ForeignKeyExists      bool `yaml:"foreign_key_exists"`
	EnabledFilter         bool `yaml:"enabled_filter"`
	IgnoreEmptyForeignKey bool `yaml:"ignore_empty_foreign_key"`
}

type TableSchema struct {
	PhysicalTable string            `yaml:"physical_table"`
	File          string            `yaml:"file"`
	PrimaryKey    string            `yaml:"primary_key"`
	EnabledField  string            `yaml:"enabled_field"`
	TimeFields    TimeFields        `yaml:"time_fields"`
	Fields        map[string]string `yaml:"fields"`
	ForeignKeys   map[string]string `yaml:"foreign_keys"`
}

type TimeFields struct {
	Start string `yaml:"start"`
	End   string `yaml:"end"`
}

type RuleLibraryConfig struct {
	Version    string               `yaml:"version"`
	RuleGroups map[string]RuleGroup `yaml:"rule_groups"`
}

type RuleGroup struct {
	Rules []RuleDef `yaml:"rules"`
}

type RuleDef struct {
	ID     string     `yaml:"id"`
	Params RuleParams `yaml:"params"`
}

type RuleParams struct {
	Required []string `yaml:"required"`
	Optional []string `yaml:"optional"`
}

type PresetsConfig struct {
	Version string                  `yaml:"version"`
	Presets map[string]PresetConfig `yaml:"presets"`
}

type PresetConfig struct {
	Name                       string         `yaml:"name"`
	Description                string         `yaml:"description"`
	RequiredParams             []string       `yaml:"required_params"`
	OptionalParams             []string       `yaml:"optional_params"`
	IncludeSchemaDefaultChecks bool           `yaml:"include_schema_default_checks"`
	Templates                  []RuleInstance `yaml:"templates"`
	Checks                     []RuleInstance `yaml:"checks"`
}

type ChecksConfig struct {
	Version     string         `yaml:"version"`
	Target      TargetConfig   `yaml:"target"`
	Preset      string         `yaml:"preset"`
	Params      map[string]any `yaml:"params"`
	ExtraChecks []RuleInstance `yaml:"extra_checks"`
	Skip        []SkipRule     `yaml:"skip"`
	Output      OutputConfig   `yaml:"output"`
}

type TargetConfig struct {
	Table string `yaml:"table"`
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
	Name  string `yaml:"name"`
}

type OutputConfig struct {
	Format      string `yaml:"format"`
	Path        string `yaml:"path"`
	PathBase    string `yaml:"path_base"`
	IncludePass bool   `yaml:"include_passed"`
}

type RuleInstance struct {
	ID         string         `yaml:"id"`
	Rule       string         `yaml:"rule"`
	Group      string         `yaml:"group"`
	With       map[string]any `yaml:"with"`
	Severity   string         `yaml:"severity"`
	Message    string         `yaml:"message"`
	Suggestion string         `yaml:"suggestion"`
}

type SkipRule struct {
	ID   string `yaml:"id"`
	Rule string `yaml:"rule"`
}

type Row map[string]string

type Table struct {
	LogicalName string
	FileName    string
	PrimaryKey  string
	Rows        []Row
	PKIndex     map[string]Row
	Indexes     map[string]map[string][]Row
}

type TableStore struct {
	Tables map[string]*Table
}

type Scope struct {
	TargetTable string
	TargetKey   string
	TargetValue string
	ActivityID  string
}

type CheckContext struct {
	Schema *SchemaConfig
	Store  *TableStore
	Target TargetConfig
	Params map[string]any
	Scope  Scope
}

type Issue struct {
	Severity      string
	RuleID        string
	RuleName      string
	Group         string
	Table         string
	RowKey        string
	Field         string
	ActualValue   string
	ExpectedValue string
	Message       string
	Suggestion    string
}

type Report struct {
	Target    TargetConfig
	Preset    string
	Summary   ReportSummary
	Issues    []Issue
	RuleCount int
}

type ReportSummary struct {
	ErrorCount int
	WarnCount  int
	PassCount  int
}
