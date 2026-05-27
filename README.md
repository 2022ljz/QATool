# Game Config Validator

Game Config Validator 是一个面向游戏活动测试场景的本地 CLI 工具，用于快速校验多张策划配置表之间的业务一致性。

它不替代导表工具，也不替代游戏运行时测试。它关注的是 QA checklist 中可以数据化的联表风险，例如活动时间、红点、签到、任务货币、兑换奖励、奖池链路和奖励物品引用是否一致。

## Features

- 读取 10 张 CSV 策划表
- 读取 4 层 YAML 校验配置
- 按 `target + preset` 定位单个活动范围
- 支持 schema 默认校验：主键唯一、外键存在
- 支持业务规则校验：
  - 生命周期窗口一致性
  - 跨表字段一致性
  - 参数化校验
  - 分组唯一、计数、最大值、聚合比较
  - 父子节点、链路环检测
- 输出 Markdown 报告
- 通过 Go 单元测试和集成测试验证示例活动

## Use Case

典型场景是 QA 在活动测试前，只指定活动 ID 和校验包：

```yaml
target:
  table: activity
  key: activity_id
  value: summer_night_2024

preset: activity_full_check
```

工具会自动加载策划表、展开 preset、执行联表规则，并生成一份可读的风险报告。

## Project Structure

```text
.
├── cmd/validator/              # CLI 入口
├── internal/validator/          # 核心实现
│   ├── checkers.go              # 业务规则 checker
│   ├── engine.go                # 规则执行引擎
│   ├── run.go                   # 一次校验的主流程
│   ├── store.go                 # CSV 加载和内存索引
│   ├── types.go                 # 配置、表、报告结构
│   └── util.go                  # 比较、时间、参数工具
├── layer/                       # 4 层 YAML 配置示例
│   ├── 01_schema.yaml
│   ├── 02_rule_library.yaml
│   ├── 03_presets.yaml
│   └── 04_checks_summer_night.yaml
├── table_config/                # 策划表 CSV 示例
├── reports/                     # 输出报告
├── PRD.md
├── Tech Design.md
├── go.mod
└── go.sum
```

## Requirements

- Go 1.23+

当前项目使用：

- `gopkg.in/yaml.v3` 解析 YAML
- Go 标准库读取 CSV、生成报告和执行测试

## Quick Start

在项目根目录运行：

```powershell
go run ./cmd/validator check `
  --schema layer/01_schema.yaml `
  --rules layer/02_rule_library.yaml `
  --presets layer/03_presets.yaml `
  --check layer/04_checks_summer_night.yaml `
  --data-dir table_config
```

如果 Windows 默认 Go build cache 有权限或目录冲突问题，可以指定当前项目内缓存：

```powershell
$env:GOCACHE='e:\GO\goproj\秋招\QATool\.gocache'
go run ./cmd/validator check --schema layer/01_schema.yaml --rules layer/02_rule_library.yaml --presets layer/03_presets.yaml --check layer/04_checks_summer_night.yaml --data-dir table_config
```

执行成功后会生成：

```text
reports/summer_night_2024.md
```

## CLI

核心命令：

```bash
validator check \
  --schema layer/01_schema.yaml \
  --rules layer/02_rule_library.yaml \
  --presets layer/03_presets.yaml \
  --check layer/04_checks_summer_night.yaml \
  --data-dir table_config \
  --out reports/summer_night_2024.md
```

参数说明：

| 参数 | 说明 |
| --- | --- |
| `--schema` | schema 配置路径 |
| `--rules` | rule library 配置路径 |
| `--presets` | preset 配置路径 |
| `--check`, `-c` | 单次 QA 校验配置路径 |
| `--data-dir` | CSV 策划表目录 |
| `--out` | 报告输出路径，优先级高于 checks YAML 中的 output.path |
| `--workers` | 并发执行规则的 worker 数量，默认 4 |

## 4 Layer Config Model

项目采用 4 层配置模型：

| 层级 | 文件 | 维护者 | 作用 |
| --- | --- | --- | --- |
| 1 | `01_schema.yaml` | 工具开发 | 声明表、字段、主键、外键、CSV 文件 |
| 2 | `02_rule_library.yaml` | 工具开发 | 定义通用规则模板和必填参数 |
| 3 | `03_presets.yaml` | 工具开发 / 资深 QA | 把规则模板组装成业务校验包 |
| 4 | `04_checks_summer_night.yaml` | QA | 指定活动、preset、业务参数和输出路径 |

普通 QA 通常只需要修改第 4 层。

## Example Report

示例数据中故意保留了若干异常，用于验证工具能力。对 `summer_night_2024` 运行 `activity_full_check` 后，报告会识别包括但不限于：

- 红点结束时间晚于活动结束时间
- 红点开放等级低于活动开放等级
- 二级红点父节点不存在
- 签到结束时间晚于活动结束时间
- 签到奖励 `day_no` 重复
- 签到奖励 `day_no` 超过 `total_days`
- 签到奖励引用不存在的 `reward_id`
- 货币周上限与 UI 展示上限不一致
- 任务产出货币与活动绑定货币不一致
- 兑换消耗货币与活动绑定货币不一致
- 兑换展示奖励与实际发放奖励不一致
- 奖池 next / preview 链路引用不存在
- 奖励引用不存在的物品

## Tests

运行测试：

```powershell
go test ./...
```

如果需要指定 Go build cache：

```powershell
$env:GOCACHE='e:\GO\goproj\秋招\QATool\.gocache'
go test ./...
```

当前测试包含：

- 比较工具单元测试
- `summer_night_2024` 集成测试
- 报告关键异常断言

## Design Boundary

本工具只检查配置数据表达出的业务风险。

不会检查：

- UI 是否真实渲染正确
- 动画是否播放正常
- 点击后游戏状态是否真实变化
- 奖励是否真的到账
- 货币是否真的扣除
- NPC、场景、特效等运行时表现

这些仍然需要进入游戏内验证。

## Roadmap

后续可以扩展：

- CSV 报告
- Excel xlsx 读取
- Web 报告页面
- 配置 diff 检查
- Git pre-commit hook
- 历史报告归档
- 缺陷系统集成
- 按活动类型自动推荐 preset

## License

当前仓库未声明 License。公开发布到 GitHub 前，建议根据项目用途补充合适的开源协议。
