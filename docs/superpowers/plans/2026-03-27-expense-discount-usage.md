# 费用管理 - 折扣使用量 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在费用管理中增加折扣使用量和折扣率字段，用总使用量（= 使用量 + 折扣使用量 × 折扣率）替代原使用量参与费用计算。

**Architecture:** 数据库增加两列（discount_usage, discount_rate），后端结构体和处理函数同步扩展，前端表格增加输入列和自动计算逻辑。存储原始值，总使用量在查询/显示时计算。

**Tech Stack:** Go（net/http, html/template）, SQLite, 原生 JS

**Spec:** `docs/superpowers/specs/2026-03-27-expense-discount-usage-design.md`

---

### Task 1: 数据库迁移 + 模型更新

**Files:**
- Modify: `db.go:58-68` (建表 + ALTER TABLE 迁移)
- Modify: `db.go:207-209` (UserExpenseInput 结构体)
- Modify: `model.go:38-46` (ExpenseUsage 结构体)

- [ ] **Step 1: model.go — ExpenseUsage 增加字段**

在 `model.go:44` 的 `Usage` 行后增加两个字段：

```go
// ExpenseUsage 用户使用量记录
type ExpenseUsage struct {
	ID             int
	ExpenseID      int
	UserID         int
	Username       string
	DisplayName    string
	Usage          float64 // 使用量
	DiscountUsage  float64 // 折扣使用量
	DiscountRate   float64 // 折扣率
	CalculatedCost float64 // 计算出的费用
}
```

- [ ] **Step 2: db.go — UserExpenseInput 增加字段**

修改 `db.go:207-209`：

```go
type UserExpenseInput struct {
	Usage         float64 // 使用量
	DiscountUsage float64 // 折扣使用量
	DiscountRate  float64 // 折扣率
}
```

- [ ] **Step 3: db.go — 增加 ALTER TABLE 迁移**

在 `db.go:68`（supplement 的 ALTER TABLE 之后）追加：

```go
db.Exec(`ALTER TABLE expense_usages ADD COLUMN discount_usage REAL NOT NULL DEFAULT 0`)
db.Exec(`ALTER TABLE expense_usages ADD COLUMN discount_rate REAL NOT NULL DEFAULT 0.5`)
```

- [ ] **Step 4: 编译验证**

Run: `go build -o gscowork .`
Expected: 编译成功，无错误

- [ ] **Step 5: Commit**

```bash
git add model.go db.go
git commit -m "feat: 增加折扣使用量和折扣率的模型与数据库迁移"
```

---

### Task 2: 后端计算与保存逻辑

**Files:**
- Modify: `handler.go:363-370` (ExpenseUserData 结构体)
- Modify: `handler.go:450-503` (handleExpenseCalculate)
- Modify: `handler.go:507-563` (handleExpenseSave)
- Modify: `db.go:235-248` (createExpenseRecord 保存逻辑)
- Modify: `db.go:285-316` (getExpenseUsages 查询逻辑)

- [ ] **Step 1: handler.go — ExpenseUserData 增加字段**

修改 `handler.go:363-370`：

```go
type ExpenseUserData struct {
	UserID        int
	Username      string
	DisplayName   string
	IsAdmin       bool
	Usage         float64
	DiscountUsage float64
	DiscountRate  float64
	TotalUsage    float64
	Cost          float64
}
```

- [ ] **Step 2: handler.go — handleExpenseCalculate 解析新字段**

修改 `handler.go:468-497`，在解析 usage 后同时解析 discount_usage 和 discount_rate，用总使用量计算费用：

```go
var totalUsage float64
usages := make(map[int]float64)
discountUsages := make(map[int]float64)
discountRates := make(map[int]float64)

for _, u := range users {
	if u.IsAdmin {
		continue
	}
	usage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("usage_%d", u.ID)), 64)
	discountUsage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("discount_usage_%d", u.ID)), 64)
	discountRate, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("discount_rate_%d", u.ID)), 64)
	usages[u.ID] = usage
	discountUsages[u.ID] = discountUsage
	discountRates[u.ID] = discountRate
	userTotalUsage := usage + discountUsage*discountRate
	totalUsage += userTotalUsage
}

results := make([]map[string]interface{}, 0)
for _, u := range users {
	if u.IsAdmin {
		continue
	}
	usage := usages[u.ID]
	userTotalUsage := usage + discountUsages[u.ID]*discountRates[u.ID]

	cost := userTotalUsage/2800.0*accountFee + serverFeePerUser
	cost = math.Round(cost*100) / 100

	results = append(results, map[string]interface{}{
		"user_id":     u.ID,
		"usage":       usage,
		"total_usage": userTotalUsage,
		"cost":        cost,
	})
}
```

- [ ] **Step 3: handler.go — handleExpenseSave 解析新字段**

修改 `handler.go:521-530`，解析 discount_usage 和 discount_rate：

```go
userInputs := make(map[int]UserExpenseInput)
for _, u := range users {
	if u.IsAdmin {
		continue
	}
	usage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("usage_%d", u.ID)), 64)
	discountUsage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("discount_usage_%d", u.ID)), 64)
	discountRate, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("discount_rate_%d", u.ID)), 64)
	userInputs[u.ID] = UserExpenseInput{
		Usage:         usage,
		DiscountUsage: discountUsage,
		DiscountRate:  discountRate,
	}
}
```

- [ ] **Step 4: db.go — createExpenseRecord 用总使用量计算费用并保存新字段**

修改 `db.go:237-244`：

```go
for userID, input := range userInputs {
	totalUsage := input.Usage + input.DiscountUsage*input.DiscountRate
	calculatedCost := totalUsage/2800.0*accountFee + serverFeePerUser

	_, err = db.Exec(
		`INSERT INTO expense_usages (expense_id, user_id, usage, supplement, calculated_cost, discount_usage, discount_rate) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		expenseID, userID, input.Usage, 0, calculatedCost, input.DiscountUsage, input.DiscountRate,
	)
	if err != nil {
		return 0, err
	}
}
```

- [ ] **Step 5: db.go — getExpenseUsages 读取新列**

修改 `db.go:286-302`，SELECT 增加 discount_usage 和 discount_rate，Scan 同步：

```go
rows, err := db.Query(`
	SELECT eu.id, eu.expense_id, eu.user_id, u.username, u.display_name,
	       eu.usage, eu.discount_usage, eu.discount_rate, eu.calculated_cost
	FROM expense_usages eu
	LEFT JOIN users u ON eu.user_id = u.id
	WHERE eu.expense_id = ?
	ORDER BY eu.calculated_cost DESC
`, expenseID)
```

Scan 行改为：

```go
rows.Scan(&eu.ID, &eu.ExpenseID, &eu.UserID, &username, &displayName,
	&eu.Usage, &eu.DiscountUsage, &eu.DiscountRate, &eu.CalculatedCost)
```

- [ ] **Step 6: 编译验证**

Run: `go build -o gscowork .`
Expected: 编译成功

- [ ] **Step 7: Commit**

```bash
git add handler.go db.go
git commit -m "feat: 后端支持折扣使用量和折扣率的计算与保存"
```

---

### Task 3: 前端表格与交互

**Files:**
- Modify: `templates/expense.html` (整个文件)

- [ ] **Step 1: 更新公式说明文本**

修改 `expense.html:40-41`：

```html
<p>计算公式：用户费用 = 总使用量 / 2800 * 账号费用 + 服务器费用 / 12 / 用户数量</p>
<p>总使用量 = 使用量 + 折扣使用量 × 折扣率</p>
<p>其中：用户数量包含admin（共{{.TotalUserCount}}人），但admin不参与使用量计算</p>
```

- [ ] **Step 2: 更新表头**

修改 `expense.html:49-53`：

```html
<thead>
    <tr>
        <th>用户</th>
        <th>使用量</th>
        <th>折扣使用量</th>
        <th>折扣率</th>
        <th>总使用量</th>
        <th>费用</th>
    </tr>
</thead>
```

- [ ] **Step 3: 更新表体 — 每用户行增加输入框和总使用量显示**

修改 `expense.html:56-71`，每行增加折扣使用量输入、折扣率输入、总使用量只读显示：

```html
{{range .Users}}
<tr data-user-id="{{.UserID}}">
    <td>{{.DisplayName}} ({{.Username}})</td>
    <td>
        <input type="number"
               name="usage_{{.UserID}}"
               class="usage-input"
               data-user-id="{{.UserID}}"
               value="{{.Usage}}"
               step="0.01"
               min="0"
               placeholder="使用量">
    </td>
    <td>
        <input type="number"
               name="discount_usage_{{.UserID}}"
               class="discount-usage-input"
               data-user-id="{{.UserID}}"
               value="{{.DiscountUsage}}"
               step="0.01"
               min="0"
               placeholder="折扣使用量">
    </td>
    <td>
        <input type="number"
               name="discount_rate_{{.UserID}}"
               class="discount-rate-input"
               data-user-id="{{.UserID}}"
               value="{{.DiscountRate}}"
               step="0.01"
               min="0"
               max="1"
               placeholder="折扣率">
    </td>
    <td class="total-usage-cell" data-user-id="{{.UserID}}">0.00</td>
    <td class="cost-cell" data-user-id="{{.UserID}}">¥0.00</td>
</tr>
{{end}}
```

- [ ] **Step 4: 更新合计行**

修改 `expense.html:73-79`：

```html
<tfoot>
    <tr class="total-row">
        <td><strong>合计</strong></td>
        <td id="total-usage">0</td>
        <td></td>
        <td></td>
        <td id="total-total-usage">0</td>
        <td id="total-cost">¥0.00</td>
    </tr>
</tfoot>
```

- [ ] **Step 5: 更新缓存函数 — 保存折扣字段**

修改 `cacheExpenseData()` 函数，收集折扣使用量和折扣率：

```javascript
function cacheExpenseData() {
    const cacheData = {
        account_fee: document.getElementById('account_fee').value,
        server_fee: document.getElementById('server_fee').value,
        usages: {},
        discount_usages: {},
        discount_rates: {},
        cached_at: new Date().toISOString()
    };

    document.querySelectorAll('.usage-input').forEach(input => {
        cacheData.usages[input.dataset.userId] = input.value;
    });
    document.querySelectorAll('.discount-usage-input').forEach(input => {
        cacheData.discount_usages[input.dataset.userId] = input.value;
    });
    document.querySelectorAll('.discount-rate-input').forEach(input => {
        cacheData.discount_rates[input.dataset.userId] = input.value;
    });

    localStorage.setItem(EXPENSE_CACHE_KEY, JSON.stringify(cacheData));
    alert('数据已缓存！下次打开页面将自动加载缓存数据。');
}
```

- [ ] **Step 6: 更新加载缓存函数 — 恢复折扣字段**

在 `loadCachedData()` 的恢复用户使用量部分之后，追加恢复折扣字段：

```javascript
if (cacheData.discount_usages) {
    for (const [userId, value] of Object.entries(cacheData.discount_usages)) {
        const input = document.querySelector(`.discount-usage-input[data-user-id="${userId}"]`);
        if (input && value) input.value = value;
    }
}
if (cacheData.discount_rates) {
    for (const [userId, value] of Object.entries(cacheData.discount_rates)) {
        const input = document.querySelector(`.discount-rate-input[data-user-id="${userId}"]`);
        if (input && value) input.value = value;
    }
}
```

- [ ] **Step 7: 更新 calculateExpense 回调 — 显示总使用量**

修改 `calculateExpense()` 的 `.then(data => { ... })` 部分，增加总使用量显示：

```javascript
.then(data => {
    document.getElementById('total-usage').textContent = data.total_usage.toFixed(2);

    let totalCost = 0;
    let totalTotalUsage = 0;
    data.results.forEach(result => {
        const costCell = document.querySelector(`.cost-cell[data-user-id="${result.user_id}"]`);
        if (costCell) {
            costCell.textContent = '¥' + result.cost.toFixed(2);
            totalCost += result.cost;
        }
        const totalUsageCell = document.querySelector(`.total-usage-cell[data-user-id="${result.user_id}"]`);
        if (totalUsageCell) {
            totalUsageCell.textContent = result.total_usage.toFixed(2);
            totalTotalUsage += result.total_usage;
        }
    });

    document.getElementById('total-total-usage').textContent = totalTotalUsage.toFixed(2);
    document.getElementById('total-cost').textContent = '¥' + totalCost.toFixed(2);
})
```

- [ ] **Step 8: 绑定新输入框的 debounce 事件**

在现有 `.usage-input` 事件绑定之后，追加折扣输入框的监听：

```javascript
document.querySelectorAll('.discount-usage-input, .discount-rate-input').forEach(input => {
    input.addEventListener('input', () => {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(calculateExpense, 300);
    });
});
```

- [ ] **Step 9: 编译验证**

Run: `go build -o gscowork .`
Expected: 编译成功

- [ ] **Step 10: Commit**

```bash
git add templates/expense.html
git commit -m "feat: 前端表格增加折扣使用量和折扣率输入"
```

---

### Task 4: 详情页显示折扣字段

**Files:**
- Modify: `templates/expense_detail.html`
- Modify: `handler.go:593-606` (handleExpenseDetail 总使用量计算)

- [ ] **Step 1: handler.go — 详情页总使用量用新公式计算**

修改 `handler.go:596-601`：

```go
var totalUsage, totalCost float64
for _, u := range usages {
	userTotal := u.Usage + u.DiscountUsage*u.DiscountRate
	totalUsage += userTotal
	totalCost += u.CalculatedCost
}
```

- [ ] **Step 2: expense_detail.html — 表头增加列**

修改 `expense_detail.html:29-33`：

```html
<thead>
    <tr>
        <th>用户</th>
        <th>使用量</th>
        <th>折扣使用量</th>
        <th>折扣率</th>
        <th>总使用量</th>
        <th>费用</th>
    </tr>
</thead>
```

- [ ] **Step 3: expense_detail.html — 表体增加列**

修改 `expense_detail.html:36-42`：

```html
{{range .Usages}}
<tr>
    <td>{{.DisplayName}} ({{.Username}})</td>
    <td>{{printf "%.2f" .Usage}}</td>
    <td>{{printf "%.2f" .DiscountUsage}}</td>
    <td>{{printf "%.2f" .DiscountRate}}</td>
    <td>{{printf "%.2f" (add3 .Usage (mul .DiscountUsage .DiscountRate))}}</td>
    <td>¥{{printf "%.2f" .CalculatedCost}}</td>
</tr>
{{end}}
```

注意：Go 的 html/template 不支持直接算术运算。需要用两种方式之一：
- **方案 A（推荐）：** 在 `ExpenseUsage` 上增加 `TotalUsage()` 方法，模板中调用 `.TotalUsage`
- **方案 B：** 注册自定义模板函数

采用方案 A，在 `model.go` 增加方法：

```go
func (e ExpenseUsage) TotalUsage() float64 {
	return e.Usage + e.DiscountUsage*e.DiscountRate
}
```

模板中改为：

```html
<td>{{printf "%.2f" .TotalUsage}}</td>
```

- [ ] **Step 4: expense_detail.html — 合计行增加列**

修改 `expense_detail.html:44-50`：

```html
<tfoot>
    <tr class="total-row">
        <td><strong>合计</strong></td>
        <td></td>
        <td></td>
        <td></td>
        <td><strong>{{printf "%.2f" .TotalUsage}}</strong></td>
        <td><strong>¥{{printf "%.2f" .TotalCost}}</strong></td>
    </tr>
</tfoot>
```

- [ ] **Step 5: 编译并手动测试**

Run: `go build -o gscowork .`
Expected: 编译成功

启动后验证：
1. 访问 /expense，填入使用量、折扣使用量、折扣率，确认总使用量和费用自动计算正确
2. 保存记录，查看详情页显示正确
3. 缓存功能正常

- [ ] **Step 6: Commit**

```bash
git add templates/expense_detail.html handler.go model.go
git commit -m "feat: 详情页展示折扣使用量和折扣率"
```

---

### Task 5: 初始化默认折扣率

**Files:**
- Modify: `handler.go:420-432` (handleExpensePage 初始化数据)
- Modify: `handler.go:535-545` (handleExpenseSave 错误时回显)

- [ ] **Step 1: handleExpensePage — 新用户默认折扣率 0.5**

修改 `handler.go:425-432`，初始化时设置 DiscountRate 默认值：

```go
expenseUsers = append(expenseUsers, ExpenseUserData{
	UserID:        u.ID,
	Username:      u.Username,
	DisplayName:   u.DisplayName,
	IsAdmin:       u.IsAdmin,
	Usage:         0,
	DiscountUsage: 0,
	DiscountRate:  0.5,
	TotalUsage:    0,
	Cost:          0,
})
```

- [ ] **Step 2: handleExpenseSave — 错误时回显折扣字段**

修改 `handler.go:541-545`，保存失败回显时包含折扣字段：

```go
expenseUsers = append(expenseUsers, ExpenseUserData{
	UserID:        u.ID,
	Username:      u.Username,
	DisplayName:   u.DisplayName,
	Usage:         input.Usage,
	DiscountUsage: input.DiscountUsage,
	DiscountRate:  input.DiscountRate,
	Cost:          0,
})
```

- [ ] **Step 3: 编译验证**

Run: `go build -o gscowork .`
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add handler.go
git commit -m "feat: 费用页面初始化默认折扣率"
```
