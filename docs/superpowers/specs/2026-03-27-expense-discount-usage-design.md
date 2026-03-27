# 费用管理 - 折扣使用量功能设计

## 概述

在费用管理界面的用户使用量表格中，增加"折扣使用量"和"折扣率"两个输入字段，引入"总使用量"概念替代原有的"使用量"参与费用计算。

## 公式变更

**原公式：**
```
用户费用 = 使用量 / 2800 × 账号费用 + 服务器费用 / 12 / 用户数量
```

**新公式：**
```
总使用量 = 使用量 + 折扣使用量 × 折扣率
用户费用 = 总使用量 / 2800 × 账号费用 + 服务器费用 / 12 / 用户数量
```

## 变更范围

### 1. 前端 - expense.html

表格从 3 列扩展为 6 列：

| 用户 | 使用量 | 折扣使用量 | 折扣率 | 总使用量 | 费用 |
|------|--------|-----------|--------|---------|------|

- 折扣使用量：number 输入框，默认 0，min=0，step=0.01
- 折扣率：number 输入框，默认 0.5，min=0，max=1，step=0.01
- 总使用量：只读显示，前端实时计算
- 合计行同步显示总使用量合计
- 公式说明文本更新
- 缓存（localStorage）覆盖新增字段的保存与恢复
- 所有新增输入框绑定 debounce 自动计算

### 2. 后端 - handler.go

- `ExpenseUserData` 增加字段：`DiscountUsage float64`、`DiscountRate float64`、`TotalUsage float64`
- `handleExpenseCalculate`：解析 `discount_usage_<id>` 和 `discount_rate_<id>` 参数，计算总使用量，用总使用量替代原使用量计算费用
- `handleExpenseSave`：解析新字段传入 `UserExpenseInput`

### 3. 数据库 - db.go

- `expense_usages` 表增加列：
  - `discount_usage REAL NOT NULL DEFAULT 0`
  - `discount_rate REAL NOT NULL DEFAULT 0.5`
- 通过 `ALTER TABLE ADD COLUMN` 迁移（与现有 supplement 列迁移模式一致）
- `UserExpenseInput` 增加 `DiscountUsage`、`DiscountRate` 字段
- `createExpenseRecord`：保存时用总使用量计算费用，同时存储 discount_usage 和 discount_rate
- `getExpenseUsages`：查询时读取新列

### 4. 模型 - model.go

- `ExpenseUsage` 增加：`DiscountUsage float64`、`DiscountRate float64`

### 5. 详情页 - expense_detail.html

表格增加列显示：折扣使用量、折扣率、总使用量。总使用量在查询时计算（usage + discount_usage * discount_rate），与数据存储策略一致。

## 数据存储策略

`expense_usages.usage` 字段继续存储**原始使用量**（非总使用量），新增 `discount_usage` 和 `discount_rate` 列。总使用量在查询时计算（usage + discount_usage * discount_rate），保持数据可追溯。

## 关于 supplement 列

`expense_usages` 表已有 `supplement` 列（始终为 0，未参与计算）。新增的折扣字段与 supplement 无关，supplement 保持现状不变。

## 默认值

- 折扣使用量：0
- 折扣率：0.5
