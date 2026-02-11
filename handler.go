package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

var tmpl *template.Template

func initTemplates() {
	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
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
				return "鸡"
			default:
				return ""
			}
		},
	}).ParseGlob("templates/*.html"))
}

// 登录页
func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if getSession(r) != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	tmpl.ExecuteTemplate(w, "login.html", nil)
}

// 提交登录
func handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := getUserByUsername(username)
	if err != nil || !checkPassword(user.Password, password) {
		tmpl.ExecuteTemplate(w, "login.html", map[string]string{"Error": "用户名或密码错误"})
		return
	}

	createSession(w, user)
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
	User     User
	Weeks    [][]CalendarDay
	IsOwner  bool
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
	tmpl.ExecuteTemplate(w, "home.html", data)
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
	tmpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
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
		tmpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": getSession(r),
			"Error":       "所有字段必填",
		})
		return
	}

	err := createUser(username, password, displayName, isAdmin)
	if err != nil {
		users, _ := getAllUsers()
		tmpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
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

	tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]interface{}{
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
		tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]interface{}{
			"User":        user,
			"CurrentUser": getSession(r),
			"Error":       "显示名称不能为空",
		})
		return
	}

	err = updateUser(id, displayName, password, isAdmin)
	if err != nil {
		user, _ := getUserByID(id)
		tmpl.ExecuteTemplate(w, "admin_edit.html", map[string]interface{}{
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
		tmpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": sess,
			"Error":       "不能删除自己",
		})
		return
	}

	err = deleteUser(id)
	if err != nil {
		users, _ := getAllUsers()
		tmpl.ExecuteTemplate(w, "admin.html", map[string]interface{}{
			"Users":       users,
			"CurrentUser": sess,
			"Error":       "删除失败",
		})
		return
	}

	http.Redirect(w, r, "/admin", http.StatusFound)
}
