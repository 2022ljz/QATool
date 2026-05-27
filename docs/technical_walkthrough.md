# Game Config Validator 技术设计与数据流讲解

这份文档用于从学习和讲解角度理解 Game Config Validator。重点不是逐行解释代码，而是说明这个项目为什么这样设计、各模块承担什么职责，以及一次校验请求从输入到报告输出经历了哪些步骤。

## 1. 项目定位

Game Config Validator 是一个本地 CLI 工具，用来校验游戏活动策划表之间的业务一致性。

它解决的问题是：很多活动配置风险并不在单张表内，而是藏在多张表之间。例如：

- 活动总表结束了，但红点表还在生效
- 任务产出的活动货币和活动绑定货币不一致
- 兑换展示奖励和实际发放奖励不一致
- 签到奖励天数超过签到配置总天数
- 奖池下一阶段配置指向了不存在的奖池
- 奖励引用了不存在的物品

这些问题通常需要 QA 手动查多张表。工具的目标就是把这类 checklist 固化成可复用规则，让 QA 只指定活动 ID 和校验包即可生成报告。

## 2. 整体架构

项目采用轻量级 Go CLI 架构，不引入 Web 服务、数据库或消息队列。

核心结构如下：

```text
cmd/validator
  main.go                 CLI 参数解析，调用 validator.Run

internal/validator
  run.go                  单次校验主流程
  types.go                配置、表、规则、报告等数据结构
  store.go                YAML/CSV 加载，TableStore 构建
  engine.go               Checker 注册与规则调度
  checkers.go             各类业务规则实现
  util.go                 参数、时间、比较等工具函数

layer
  01_schema.yaml          表结构和外键声明
  02_rule_library.yaml    规则模板声明
  03_presets.yaml         业务校验包
  04_checks_summer_night.yaml

table_config
  *.csv                   策划表测试数据

scripts
  check.sh                默认运行入口
```

可以把它理解成五个核心组件：

| 组件 | 职责 |
| --- | --- |
| Loader | 读取 YAML 和 CSV，并在 CSV 读取阶段按目标活动过滤行 |
| TableStore | 保存过滤后的 CSV 数据并建立索引 |
| Resolver | 根据 preset、extra_checks、skip 得到本次要执行的规则 |
| Checker Engine | 按规则类型调度具体 checker |
| Reporter | 把 Issue 排序并输出 Markdown |

## 3. 为什么使用 4 层配置

项目最重要的设计是 4 层配置模型。它把“工具如何理解表”和“QA 本次要检查什么”分开。

### 3.1 Layer 1: schema

文件：`layer/01_schema.yaml`

这一层告诉工具：

- 有哪些逻辑表
- 每张表对应哪个 CSV 文件
- 主键字段是什么
- 哪些字段是外键
- 哪些字段是开始/结束时间
- 哪个字段表示 enabled

示例：

```yaml
activity:
  file: activity_config.csv
  primary_key: activity_id
  time_fields:
    start: start_time
    end: end_time
```

这层由工具开发维护，普通 QA 通常不需要改。

### 3.2 Layer 2: rule_library

文件：`layer/02_rule_library.yaml`

这一层定义“有哪些规则模板”，例如：

- `time_window_within`
- `same_table_field_match`
- `field_match_via_fk`
- `field_unique_in_group`
- `aggregate_compare`
- `parent_exists`
- `no_cycle`

它只声明规则需要哪些参数，不绑定具体活动。

### 3.3 Layer 3: presets

文件：`layer/03_presets.yaml`

这一层把规则模板组合成业务校验包。

例如 `activity_full_check` 会包含：

- 活动、签到、红点、任务、兑换、奖池的时间窗口检查
- 红点开放等级检查
- 签到奖励天数检查
- 货币周上限和 UI 上限检查
- 任务产出货币检查
- 兑换展示/实际奖励检查
- 奖池链路检查

这层适合由工具开发和资深 QA 共同维护。

Layer 3 同时声明该 preset 对 QA 暴露的参数契约，例如：

```yaml
required_params:
  - currency_id
  - weekly_limit
  - activity_weeks
optional_params: []
```

普通 QA 不需要反查 preset 里的每一条规则来判断要填哪些参数；工具会根据这里的声明检查 layer 4 是否缺参。

### 3.4 Layer 4: checks

文件：`layer/04_checks_summer_night.yaml`

这一层是 QA 的入口。普通使用只需要关心：

```yaml
target:
  table: activity
  key: activity_id
  value: summer_night_2024

preset: activity_full_check
```

还可以通过 `params` 提供业务参数，例如：

```yaml
params:
  currency_id: NIGHT_JADE
  weekly_limit: 120
  expected_reset_weekday: Monday
  activity_weeks: 2
```

其中 `currency_id`、`weekly_limit`、`activity_weeks` 来自 `activity_full_check` 的 `required_params`；`expected_reset_weekday` 来自本次 layer 4 额外补充的 `extra_checks`。如果写了未声明、也未被任何规则引用的参数，工具会直接报错，避免示例字段长期漂移。

## 4. 核心数据结构

### 4.1 TableStore

CSV 加载时不是无条件把所有行都放入内存。`LoadStore` 会先根据 layer 4 的 target 构造行过滤器，然后逐行读取 CSV：

- 根表 `activity` 只保留目标活动行
- 带有 `activity_id` 字段的表只保留 `activity_id == target.value` 的行
- 没有 `activity_id` 的通用引用表，例如 `reward`、`item`，当前仍保留全量，避免间接外键引用无法解析

过滤后的数据会进入内存表结构：

```go
type TableStore struct {
    Tables map[string]*Table
}
```

每张表包含：

```go
type Table struct {
    LogicalName string
    PrimaryKey  string
    Rows        []Row
    PKIndex     map[string]Row
    Indexes     map[string]map[string][]Row
}
```

其中：

- `Rows` 保存通过目标活动过滤后的行
- `PKIndex` 支持按主键快速查找
- `Indexes` 支持按常用字段快速查找，例如 `activity_id`

这样做的好处是：不用数据库，也能高效完成联表查询；同时单次活动校验不会把其他活动的大量行长期留在内存里。

### 4.2 RuleInstance

preset 展开后，每条规则都是一个 `RuleInstance`：

```go
type RuleInstance struct {
    ID         string
    Rule       string
    Group      string
    With       map[string]any
    Severity   string
    Message    string
    Suggestion string
}
```

其中：

- `Rule` 指向规则类型，例如 `time_window_within`
- `Group` 表示规则类别，例如 `lifecycle`
- `With` 是规则参数
- `Severity` 决定报告级别
- `Message` 是报告里的问题描述

### 4.3 Issue

所有 checker 发现的问题都会转换成统一 Issue：

```go
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
```

统一 Issue 的好处是：不同 checker 的输出可以用同一套排序和报告逻辑处理。

## 5. 一次校验的数据流

下面是一条完整数据流，从运行脚本到生成报告。

```text
bash scripts/check.sh
        |
        v
go run ./cmd/validator check ...
        |
        v
cmd/validator/main.go
        |
        v
validator.Run
        |
        +--> 读取 checks.yaml
        +--> 读取 schema.yaml
        +--> 读取 rule_library.yaml
        +--> 读取 presets.yaml
        |
        v
LoadStore
        |
        +--> 根据 schema 找到 CSV 文件
        +--> 按表并发打开 CSV
        +--> 单张 CSV 顺序流式读取
        +--> 清理 UTF-8 BOM 表头
        +--> 不属于 target activity_id 的活动域行直接丢弃
        +--> 对保留下来的行构建主键索引和 activity_id 索引
        |
        v
定位 target activity
        |
        v
执行 schema default checks
        |
        +--> primary_key_unique
        +--> foreign_key_exists
        |
        v
prepareRules
        |
        +--> 展开 preset templates
        +--> 合并 extra_checks
        +--> 应用 skip
        +--> 校验 preset required_params
        +--> 扫描 param: xxx 和 $params.xxx 引用
        +--> 解析 $params.xxx
        +--> 校验 rule_library required 参数
        |
        v
Engine.Run
        |
        +--> 根据 rule 名称找到 checker
        +--> worker pool 并发执行规则
        +--> 收集 Issue
        |
        v
排序 Issue
        |
        v
writeMarkdown
        |
        v
reports/summer_night_2024.md
```

## 6. 活动范围过滤

工具默认只检查目标活动相关数据，不扫全量活动。这个过滤现在前移到了 CSV 读取阶段，而不是等所有表加载进内存后再过滤。

核心策略：

- `activity` 表只加载并检查目标 `activity_id`
- 如果表中有 `activity_id` 字段，只加载并检查 `activity_id == target.value` 的行
- 没有 `activity_id` 的下游表，例如 `reward`、`item`，主要通过外键引用或 schema 默认校验触达

这样既可以减少报告噪音，让 QA 只看到当前活动相关问题，也可以减少大表场景下的内存占用。

## 7. Checker 设计

Checker 是规则执行器。每类规则都有一个 checker 函数。

注册逻辑在 `engine.go` 中：

```go
r.Register("time_window_within", checkerFunc(checkTimeWindowWithin))
r.Register("same_table_field_match", checkerFunc(checkSameTableFieldMatch))
r.Register("parent_exists", checkerFunc(checkParentExists))
```

执行时 Engine 根据 `RuleInstance.Rule` 找到对应 checker。

这种设计的优点：

- Engine 不需要知道每条规则的业务细节
- 新增规则时只需要新增 checker 并注册
- YAML 只负责传参，不承载复杂 DSL

## 8. 典型规则讲解

### 8.1 time_window_within

用于检查子表时间是否落在父表生命周期内。

示例：

```yaml
rule: time_window_within
with:
  child_table: redpoint
  parent_table: activity
  fk_field: activity_id
```

含义：

```text
redpoint.start_time >= activity.start_time
redpoint.end_time   <= activity.end_time
```

能发现活动结束后红点仍然存在的问题。

### 8.2 same_table_field_match

用于检查同一行内两个字段是否一致。

示例：

```yaml
rule: same_table_field_match
with:
  table: exchange
  field_a: display_reward_id
  field_b: actual_reward_id
```

能发现展示奖励和实际发放奖励不一致的问题。

### 8.3 field_match_via_fk

用于检查子表字段是否等于父表字段。

示例：

```yaml
rule: field_match_via_fk
with:
  child_table: task
  parent_table: activity
  fk_field: activity_id
  child_field: reward_currency_id
  parent_field: currency_id
```

含义：

```text
task.reward_currency_id == activity.currency_id
```

能发现任务产出货币和活动绑定货币不一致的问题。

### 8.4 field_unique_in_group

用于检查同一分组内字段不能重复。

示例：

```yaml
rule: field_unique_in_group
with:
  table: signin_reward
  group_by: signin_id
  unique_field: day_no
```

能发现同一个签到活动下 `day_no` 重复的问题。

### 8.5 aggregate_compare

用于聚合比较。

示例：

```yaml
rule: aggregate_compare
with:
  left_table: task
  left_field: reward_currency_count
  left_agg: sum
  right_table: exchange
  right_field: cost_count
  right_agg: sum
  operator: ">="
```

可用于发现理论产出是否覆盖兑换消耗。

## 9. Schema 默认校验

当 preset 设置：

```yaml
include_schema_default_checks: true
```

工具会执行 schema 默认校验。

当前实现包括：

| 校验 | 说明 |
| --- | --- |
| `primary_key_unique` | 检查每张表主键是否重复 |
| `foreign_key_exists` | 检查 schema 中声明的外键是否存在 |

报告中的：

```text
foreign key reference must exist
```

就是 `foreign_key_exists` 产生的默认 Issue。

## 10. 报告生成逻辑

报告输出由 `writeMarkdown` 完成。

输出文件路径优先级：

1. CLI 参数 `--out`
2. checks YAML 中的 `output.path`
3. 默认 `reports/<activity_id>.md`

Issue 排序规则：

1. `ERROR` 在 `WARN` 前
2. 按 `group`
3. 按 `table`
4. 按 `row_key`
5. 按 `rule_id`

这样可以保证同一份数据每次输出稳定，便于对比和回归。

## 11. 并发设计

项目中并发使用得比较克制，主要在两个地方：

### 11.1 CSV 加载

多张 CSV 互不依赖，可以按表并发加载。单张 CSV 内部不做 goroutine 分片，而是顺序流式读取；每读一行就判断是否属于目标活动，不属于目标活动的活动域数据不会进入 `Rows`、`PKIndex` 或二级索引。

当前没有把 CSV 读取和规则执行做成流水线。流程仍然是先完成本次需要的数据加载和索引构建，再执行规则。

### 11.2 规则执行

preset 展开后的规则多数只读 `TableStore`，因此可以通过 worker pool 并发执行。

注意：项目没有做行级 goroutine，也没有引入数据库事务。数据加载完成后 `TableStore` 只读，避免并发写入风险。

### 11.3 数据裁剪边界

当前裁剪策略优先保证正确性：

- `activity`、`signin`、`redpoint`、`task`、`currency`、`exchange`、`reward_pool`、`signin_reward` 这类带 `activity_id` 的表可以在读取阶段按目标活动过滤
- `reward`、`item` 这类没有 `activity_id` 的表暂时全量加载，因为它们通常通过奖励 ID、物品 ID 被间接引用

如果线上表规模继续扩大，下一步可以做两阶段加载：先读取目标活动域表，收集 `reward_id`、`item_id`、`pool_id` 等引用集合，再用这些 ID 精确加载无 `activity_id` 的共享表。

## 12. 扩展方式

如果要新增一种业务规则，推荐流程：

1. 在 `checkers.go` 中新增 checker 函数
2. 在 `engine.go` 的 `NewRegistry` 中注册规则名
3. 在 `02_rule_library.yaml` 中声明规则模板和参数
4. 在 `03_presets.yaml` 中把规则加入某个 preset
5. 补充测试

这条路径保持了代码和配置的边界：

- Go 代码负责规则逻辑
- YAML 负责规则组合和参数传递

## 13. 当前边界

工具只能发现配置数据风险，不能证明游戏运行时行为一定正确。

它不能检查：

- UI 是否真实渲染
- 动画是否播放
- 点击后是否弹窗
- 奖励是否真实到账
- 货币是否真实扣除
- NPC、场景、特效是否正常

这些仍然需要游戏内测试。

## 14. 总结

这个项目的技术核心不是框架，而是一个轻量规则引擎：

```text
CSV/YAML Loader
        |
TableStore + Index
        |
Preset Resolver
        |
Checker Registry
        |
Rule Engine
        |
Markdown Reporter
```

它通过 4 层配置降低 QA 使用成本，通过 checker 注册机制保持扩展性，通过流式读取、活动范围过滤和内存索引保证执行简单高效。
