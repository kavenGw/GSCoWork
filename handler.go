package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"time"
)

var templates map[string]*template.Template

func initTemplates() {
	funcMap := template.FuncMap{
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"statusClass": func(s int) string {
			switch s {
			case StatusRest:
				return "rest"
			case StatusFire:
				return "fire"
			default:
				return "default"
			}
		},
		"statusLabel": func(s int) string {
			switch s {
			case StatusRest:
				return "休"
			case StatusFire:
				return "🐮🐴"
			default:
				return ""
			}
		},
	}

	templates = make(map[string]*template.Template)

	// 独立页面（无 layout）
	templates["login.html"] = template.Must(
		template.New("login.html").Funcs(funcMap).ParseFiles("templates/login.html"),
	)

	// 使用 layout 的页面，每个单独解析避免 content 定义冲突
	layoutPages := []string{
		"home.html", "admin.html", "admin_edit.html",
		"expense.html", "expense_history.html", "expense_detail.html",
	}
	for _, page := range layoutPages {
		templates[page] = template.Must(
			template.New("").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/"+page),
		)
	}
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	t, ok := templates[name]
	if !ok {
		http.Error(w, "模板未找到: "+name, http.StatusInternalServerError)
		return
	}
	t.ExecuteTemplate(w, name, data)
}

// 登录页
func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if getSession(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderTemplate(w, "login.html", nil)
}

// 提交登录
func handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	rememberMe := r.FormValue("remember") == "on"

	user, err := getUserByUsername(username)
	if err != nil || !checkPassword(user.Password, password) {
		renderTemplate(w, "login.html", map[string]string{"Error": "用户名或密码错误"})
		return
	}

	createSession(w, user, rememberMe)
	http.Redirect(w, r, "/", http.StatusFound)
}

// 退出登录
func handleLogout(w http.ResponseWriter, r *http.Request) {
	destroySession(w, r)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// 日历数据
type CalendarDay struct {
	Day    int
	Date   string
	Status int
}

type UserCalendar struct {
	User    User
	Weeks   [][]CalendarDay
	IsOwner bool
}

type HomeData struct {
	CurrentUser *Session
	Calendars   []UserCalendar
	Year        int
	Month       int
	MonthName   string
	PrevMonth   string
	NextMonth   string
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)

	// 解析月份参数
	now := time.Now()
	year, month := now.Year(), int(now.Month())
	if m := r.URL.Query().Get("month"); m != "" {
		fmt.Sscanf(m, "%d-%d", &year, &month)
	}

	// 上下月
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	prev := t.AddDate(0, -1, 0)
	next := t.AddDate(0, 1, 0)

	users, _ := getAllUsers()

	var calendars []UserCalendar
	for _, u := range users {
		monthStr := fmt.Sprintf("%04d-%02d", year, month)
		schedules, _ := getSchedules(u.ID, monthStr)

		// 构建日历网格
		firstDay := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
		weekday := int(firstDay.Weekday()) // 0=Sunday
		daysInMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local).Day()

		var days []CalendarDay
		// 填充前置空白
		for i := 0; i < weekday; i++ {
			days = append(days, CalendarDay{})
		}
		// 填充日期
		for d := 1; d <= daysInMonth; d++ {
			dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, d)
			status := StatusDefault
			if s, ok := schedules[dateStr]; ok {
				status = s
			}
			days = append(days, CalendarDay{Day: d, Date: dateStr, Status: status})
		}
		// 补齐最后一周
		for len(days)%7 != 0 {
			days = append(days, CalendarDay{})
		}

		var weeks [][]CalendarDay
		for i := 0; i < len(days); i += 7 {
			weeks = append(weeks, days[i:i+7])
		}

		calendars = append(calendars, UserCalendar{
			User:    u,
			Weeks:   weeks,
			IsOwner: u.ID == sess.UserID,
		})
	}

	data := HomeData{
		CurrentUser: sess,
		Calendars:   calendars,
		Year:        year,
		Month:       month,
		MonthName:   fmt.Sprintf("%d年%d月", year, month),
		PrevMonth:   fmt.Sprintf("%04d-%02d", prev.Year(), int(prev.Month())),
		NextMonth:   fmt.Sprintf("%04d-%02d", next.Year(), int(next.Month())),
	}
	renderTemplate(w, "home.html", data)
}

// 更新日程状态
func handleScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	date := r.FormValue("date")
	userID, _ := strconv.Atoi(r.FormValue("user_id"))

	if userID != sess.UserID {
		http.Error(w, "无权操作", http.StatusForbidden)
		return
	}

	// 循环切换状态
	current := getScheduleStatus(userID, date)
	next := current + 1
	if next > StatusFire {
		next = StatusDefault
	}

	setSchedule(userID, date, next)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": next,
	})
}

// 后台管理页
func handleAdminPage(w http.ResponseWriter, r *http.Request) {
	users, _ := getAllUsers()
	renderTemplate(w, "admin.html", map[string]interface{}{
		"Users":       users,
		"CurrentUser": getSession(r),
	})
}

// 创建用户
func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	displayName := r.FormValue("display_name")
	isAdmin := r.FormValue("is_admin") == "on"

	if username == "" || password == "" || displayName == "" {
		users, _ := getAllUsers()
		renderTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": getSession(r),
			"Error":       "所有字段必填",
		})
		return
	}

	err := createUser(username, password, displayName, isAdmin)
	if err != nil {
		users, _ := getAllUsers()
		renderTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": getSession(r),
			"Error":       "创建失败：用户名可能已存在",
		})
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 编辑用户页面
func handleEditUserPage(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	user, err := getUserByID(id)
	if err != nil {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	renderTemplate(w, "admin_edit.html", map[string]interface{}{
		"User":        user,
		"CurrentUser": getSession(r),
	})
}

// 更新用户
func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	displayName := r.FormValue("display_name")
	password := r.FormValue("password") // 可选，留空不修改
	isAdmin := r.FormValue("is_admin") == "on"

	if displayName == "" {
		user, _ := getUserByID(id)
		renderTemplate(w, "admin_edit.html", map[string]interface{}{
			"User":        user,
			"CurrentUser": getSession(r),
			"Error":       "显示名称不能为空",
		})
		return
	}

	err = updateUser(id, displayName, password, isAdmin)
	if err != nil {
		user, _ := getUserByID(id)
		renderTemplate(w, "admin_edit.html", map[string]interface{}{
			"User":        user,
			"CurrentUser": getSession(r),
			"Error":       "更新失败",
		})
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}

// 删除用户
func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/admin", http.StatusFound)
		return
	}

	// 防止删除自己
	if id == sess.UserID {
		users, _ := getAllUsers()
		renderTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": sess,
			"Error":       "不能删除自己",
		})
		return
	}

	err = deleteUser(id)
	if err != nil {
		users, _ := getAllUsers()
		renderTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": sess,
			"Error":       "删除失败",
		})
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}

// ========== 费用管理 ==========

// 费用展示数据
type ExpenseUserData struct {
	UserID      int
	Username    string
	DisplayName string
	IsAdmin     bool
	Usage       float64
	PrevUsage   float64 // 上周期使用量（用于计算本周期实际使用量 = Usage - PrevUsage）
	Cost        float64
}

type ExpensePageData struct {
	CurrentUser    *Session
	Users          []ExpenseUserData
	TotalUserCount int     // 包含admin在内的所有用户数（用于计算服务器费用分摊）
	AccountFee     float64
	ServerFee      float64
	TotalUsage     float64
	StartDate      string
	EndDate        string
	Error          string
	Success        string
}

// 费用页面
func handleExpensePage(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	users, _ := getAllUsers()

	// 默认日期范围：根据上一个周期自动计算
	// 例如上一个周期是1.12 - 2.12，下一个就是2.12 - 3.12
	var startDate, endDate string
	latestRecord, err := getLatestExpenseRecord()
	if err == nil && latestRecord != nil {
		// 有上一个周期，根据上一个周期计算
		// 新的开始日期 = 上一个周期的结束日期
		startDate = latestRecord.EndDate

		// 新的结束日期 = 上一个周期的结束日期 + 1个月
		prevEndDate, parseErr := time.Parse("2006-01-02", latestRecord.EndDate)
		if parseErr == nil {
			nextEndDate := prevEndDate.AddDate(0, 1, 0)
			endDate = nextEndDate.Format("2006-01-02")
		} else {
			// 解析失败，使用当月
			now := time.Now()
			endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local).Format("2006-01-02")
		}
	} else {
		// 没有上一个周期，默认当月
		now := time.Now()
		startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
		endDate = time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	}

	// 获取上个周期的使用量数据（用于自动填充上周期字段）
	prevUsages := make(map[int]float64)
	if latestRecord != nil {
		usages, err := getExpenseUsages(latestRecord.ID)
		if err == nil {
			for _, u := range usages {
				prevUsages[u.UserID] = u.Usage
			}
		}
	}

	// 统计所有用户数量（包含admin，用于服务器费用分摊）
	totalUserCount := len(users)

	// 只显示非admin用户
	var expenseUsers []ExpenseUserData
	for _, u := range users {
		if u.IsAdmin {
			continue // 跳过admin用户
		}
		expenseUsers = append(expenseUsers, ExpenseUserData{
			UserID:      u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			IsAdmin:     u.IsAdmin,
			Usage:       0,
			PrevUsage:   prevUsages[u.ID], // 自动填充上周期使用量
			Cost:        0,
		})
	}

	data := ExpensePageData{
		CurrentUser:    sess,
		Users:          expenseUsers,
		TotalUserCount: totalUserCount,
		AccountFee:     DefaultAccountFee,
		ServerFee:      DefaultServerFee,
		TotalUsage:     0,
		StartDate:      startDate,
		EndDate:        endDate,
	}

	renderTemplate(w, "expense.html", data)
}

// 计算费用（AJAX）
func handleExpenseCalculate(w http.ResponseWriter, r *http.Request) {
	accountFee, _ := strconv.ParseFloat(r.FormValue("account_fee"), 64)
	serverFee, _ := strconv.ParseFloat(r.FormValue("server_fee"), 64)
	totalUserCount, _ := strconv.Atoi(r.FormValue("total_user_count"))

	users, _ := getAllUsers()

	// 确保用户数量至少为1
	if totalUserCount == 0 {
		totalUserCount = len(users)
	}
	if totalUserCount == 0 {
		totalUserCount = 1
	}

	// 计算每个用户的服务器费用分摊部分（所有用户平均分摊，包含admin）
	serverFeePerUser := serverFee / 12.0 / float64(totalUserCount)

	var totalActualUsage float64
	rawUsages := make(map[int]float64)    // 总额
	prevUsages := make(map[int]float64)   // 上周期
	actualUsages := make(map[int]float64) // 实际使用量 = 总额 - 上周期

	// 只处理非admin用户
	for _, u := range users {
		if u.IsAdmin {
			continue
		}
		rawUsage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("usage_%d", u.ID)), 64)
		prevUsage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("prev_usage_%d", u.ID)), 64)

		rawUsages[u.ID] = rawUsage
		prevUsages[u.ID] = prevUsage
		// 实际使用量 = 总额 - 上周期
		actualUsage := rawUsage - prevUsage
		if actualUsage < 0 {
			actualUsage = 0
		}
		actualUsages[u.ID] = actualUsage
		totalActualUsage += actualUsage
	}

	results := make([]map[string]interface{}, 0)
	for _, u := range users {
		if u.IsAdmin {
			continue
		}
		actualUsage := actualUsages[u.ID]

		// 公式：(使用量 - 上周期) / 2800 * 账号费用 + 服务器费用/12/用户数量
		cost := actualUsage/2800.0*accountFee + serverFeePerUser
		cost = math.Round(cost*100) / 100 // 保留两位小数

		results = append(results, map[string]interface{}{
			"user_id":      u.ID,
			"usage":        rawUsages[u.ID],
			"prev_usage":   prevUsages[u.ID],
			"actual_usage": actualUsage,
			"cost":         cost,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_usage": totalActualUsage,
		"results":     results,
	})
}

// 保存费用记录
func handleExpenseSave(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)

	startDate := r.FormValue("start_date")
	endDate := r.FormValue("end_date")
	accountFee, _ := strconv.ParseFloat(r.FormValue("account_fee"), 64)
	serverFee, _ := strconv.ParseFloat(r.FormValue("server_fee"), 64)

	users, _ := getAllUsers()

	// 统计所有用户数量（包含admin，用于服务器费用分摊）
	totalUserCount := len(users)

	// 只处理非admin用户
	userInputs := make(map[int]UserExpenseInput)
	for _, u := range users {
		if u.IsAdmin {
			continue
		}
		usage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("usage_%d", u.ID)), 64)
		prevUsage, _ := strconv.ParseFloat(r.FormValue(fmt.Sprintf("prev_usage_%d", u.ID)), 64)
		userInputs[u.ID] = UserExpenseInput{
			Usage:     usage,
			PrevUsage: prevUsage,
		}
	}

	_, err := createExpenseRecord(startDate, endDate, accountFee, serverFee, userInputs, totalUserCount)
	if err != nil {
		// 重新渲染页面并显示错误
		var expenseUsers []ExpenseUserData
		for _, u := range users {
			if u.IsAdmin {
				continue
			}
			input := userInputs[u.ID]
			expenseUsers = append(expenseUsers, ExpenseUserData{
				UserID:      u.ID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				Usage:       input.Usage,
			})
		}

		data := ExpensePageData{
			CurrentUser:    sess,
			Users:          expenseUsers,
			TotalUserCount: totalUserCount,
			AccountFee:     accountFee,
			ServerFee:      serverFee,
			StartDate:      startDate,
			EndDate:        endDate,
			Error:          "保存失败：" + err.Error(),
		}
		renderTemplate(w, "expense.html", data)
		return
	}

	http.Redirect(w, r, "/expense/history", http.StatusFound)
}

// 费用历史记录
func handleExpenseHistory(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	records, _ := getAllExpenseRecords()

	renderTemplate(w, "expense_history.html", map[string]interface{}{
		"CurrentUser": sess,
		"Records":     records,
	})
}

// 费用记录详情
func handleExpenseDetail(w http.ResponseWriter, r *http.Request) {
	sess := getSession(r)
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/expense/history", http.StatusFound)
		return
	}

	record, err := getExpenseRecordByID(id)
	if err != nil {
		http.Redirect(w, r, "/expense/history", http.StatusFound)
		return
	}

	usages, _ := getExpenseUsages(id)

	// 计算总使用量和总费用
	var totalUsage, totalCost float64
	for _, u := range usages {
		totalUsage += u.Usage
		totalCost += u.CalculatedCost
	}

	renderTemplate(w, "expense_detail.html", map[string]interface{}{
		"CurrentUser": sess,
		"Record":      record,
		"Usages":      usages,
		"TotalUsage":  totalUsage,
		"TotalCost":   math.Round(totalCost*100) / 100,
	})
}

// 删除费用记录
func handleExpenseDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/expense/history", http.StatusFound)
		return
	}

	deleteExpenseRecord(id)
	http.Redirect(w, r, "/expense/history", http.StatusFound)
}

// 费用管理页面添加用户
func handleExpenseUserAdd(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	displayName := r.FormValue("display_name")

	if username == "" || password == "" || displayName == "" {
		http.Redirect(w, r, "/expense", http.StatusFound)
		return
	}

	err := createUser(username, password, displayName, false)
	if err != nil {
		http.Redirect(w, r, "/expense", http.StatusFound)
		return
	}

	http.Redirect(w, r, "/expense", http.StatusFound)
}

// 费用管理页面删除用户
func handleExpenseUserDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/expense", http.StatusFound)
		return
	}

	// 不允许删除自己
	sess := getSession(r)
	if sess != nil && sess.UserID == id {
		http.Redirect(w, r, "/expense", http.StatusFound)
		return
	}

	deleteUser(id)
	http.Redirect(w, r, "/expense", http.StatusFound)
}
