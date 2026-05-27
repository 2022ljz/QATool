# Game Activity Config CSV Fixtures

这组 CSV 是为“基于 checklist 的策划配置表业务级联表校验工具”准备的模拟数据。

## 表清单
1. activity_config.csv
2. redpoint_config.csv
3. signin_config.csv
4. signin_reward.csv
5. currency_config.csv
6. task_config.csv
7. exchange_config.csv
8. reward_pool.csv
9. reward_config.csv
10. item_config.csv

## 主要测试活动
- summer_night_2024：一夏夜梦谭，用于覆盖活动时间、红点、签到、夜之玉、兑换、奖池等多数规则。
- qinghui_signin_2024：清辉梦旅签到，用于正常签到链路对照。
- moon_pool_2024：月夜奖池，用于奖池链路对照。

## 有意保留的异常数据，方便测试工具产生报告
- redpoint_config.csv：RP_SUMMER_REWARD / RP_SUMMER_POOL 的 end_time 晚于活动结束时间。
- redpoint_config.csv：RP_SUMMER_LOW_LEVEL 的 open_level 低于活动开放等级。
- redpoint_config.csv：RP_SUMMER_ORPHAN_CHILD 的 parent_redpoint_id 不存在。
- signin_config.csv：SIG_SUMMER_NIGHT 的 end_time 晚于活动结束时间。
- signin_reward.csv：summer_night_2024 存在重复 day_no=3、day_no=8 超过 total_days、RWD_MISSING 不存在。
- currency_config.csv：NIGHT_JADE 的 weekly_limit=120，但 ui_display_limit=100。
- task_config.csv：TASK_SUMMER_WRONG_REFRESH 周刷新日为 Tuesday，和货币 Monday 重置不一致。
- task_config.csv：TASK_SUMMER_BAD_CURRENCY 产出 WRONG_CURRENCY，和活动货币不一致。
- exchange_config.csv：EX_SUMMER_CORE_1 的 display_reward_id 与 actual_reward_id 不一致。
- exchange_config.csv：EX_SUMMER_BAD_CURRENCY 消耗 MOON_TOKEN，和活动货币 NIGHT_JADE 不一致。
- reward_pool.csv：POOL_SUMMER_STAGE2 的 next_pool_id / preview_pool_id 指向不存在的 POOL_SUMMER_STAGE3_MISSING。
- reward_pool.csv：POOL_SUMMER_STAGE2 的 trigger_redpoint_id 指向不存在的 RP_POOL_MISSING。
- reward_config.csv：RWD_POOL_STAGE2_ITEM 引用不存在的 ITEM_BROKEN_REF。

## 建议运行入口
QA 侧可以用：
```yaml
target:
  type: activity
  id: summer_night_2024
preset: activity_full_check
params:
  currency_id: NIGHT_JADE
  weekly_limit: 120
```
