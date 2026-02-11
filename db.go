package main

import (
	"database/sql"
	"log"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func initDB(dbPath string) {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// 建表
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		display_name TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL REFERENCES users(id),
		date TEXT NOT NULL,
		status INTEGER NOT NULL DEFAULT 1
	)`)

	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_schedules_user_date ON schedules(user_id, date)`)

	// 费用记录表
	db.Exec(`CREATE TABLE IF NOT EXISTS expense_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_date TEXT NOT NULL,
		end_date TEXT NOT NULL,
		account_fee REAL NOT NULL DEFAULT 550,
		server_fee REAL NOT NULL DEFAULT 99,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	// 用户使用量表
	db.Exec(`CREATE TABLE IF NOT EXISTS expense_usages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		expense_id INTEGER NOT NULL REFERENCES expense_records(id),
		user_id INTEGER NOT NULL,
		usage REAL NOT NULL DEFAULT 0,
		calculated_cost REAL NOT NULL DEFAULT 0
	)`)

	// 创建默认 admin 账号
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		db.Exec("INSERT INTO users (username, password, display_name, is_admin) VALUES (?, ?, ?, ?)",
			"admin", string(hash), "管理员", true)
		log.Println("默认 admin 账号已创建 (admin / admin123)")
	}
}

func getUserByUsername(username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, username, password, display_name, is_admin, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.Password, &u.DisplayName, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func getAllUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password, display_name, is_admin, created_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Password, &u.DisplayName, &u.IsAdmin, &u.CreatedAt)
		users = append(users, u)
	}
	return users, nil
}

func createUser(username, password, displayName string, isAdmin bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO users (username, password, display_name, is_admin) VALUES (?, ?, ?, ?)",
		username, string(hash), displayName, isAdmin,
	)
	return err
}

func getSchedules(userID int, month string) (map[string]int, error) {
	// month 格式: "2026-02"
	rows, err := db.Query(
		"SELECT date, status FROM schedules WHERE user_id = ? AND date LIKE ?",
		userID, month+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var date string
		var status int
		rows.Scan(&date, &status)
		result[date] = status
	}
	return result, nil
}

func setSchedule(userID int, date string, status int) error {
	_, err := db.Exec(
		`INSERT INTO schedules (user_id, date, status) VALUES (?, ?, ?)
		 ON CONFLICT(user_id, date) DO UPDATE SET status = ?`,
		userID, date, status, status,
	)
	return err
}

func getScheduleStatus(userID int, date string) int {
	var status int
	err := db.QueryRow("SELECT status FROM schedules WHERE user_id = ? AND date = ?", userID, date).Scan(&status)
	if err != nil {
		return StatusDefault
	}
	return status
}

func getUserByID(id int) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		"SELECT id, username, password, display_name, is_admin, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Password, &u.DisplayName, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func updateUser(id int, displayName string, password string, isAdmin bool) error {
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = db.Exec(
			"UPDATE users SET display_name = ?, password = ?, is_admin = ? WHERE id = ?",
			displayName, string(hash), isAdmin, id,
		)
		return err
	}
	_, err := db.Exec(
		"UPDATE users SET display_name = ?, is_admin = ? WHERE id = ?",
		displayName, isAdmin, id,
	)
	return err
}

func deleteUser(id int) error {
	// 先删除用户的日程数据
	_, err := db.Exec("DELETE FROM schedules WHERE user_id = ?", id)
	if err != nil {
		return err
	}
	// 再删除用户
	_, err = db.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// ========== 费用相关 ==========

// 创建费用记录
func createExpenseRecord(startDate, endDate string, accountFee, serverFee float64, usages map[int]float64) (int64, error) {
	result, err := db.Exec(
		`INSERT INTO expense_records (start_date, end_date, account_fee, server_fee) VALUES (?, ?, ?, ?)`,
		startDate, endDate, accountFee, serverFee,
	)
	if err != nil {
		return 0, err
	}

	expenseID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	// 计算总使用量
	var totalUsage float64
	for _, u := range usages {
		totalUsage += u
	}

	// 获取用户数量
	userCount := len(usages)
	if userCount == 0 {
		userCount = 1
	}

	// 保存每个用户的使用量和计算的费用
	for userID, usage := range usages {
		var calculatedCost float64
		if totalUsage > 0 {
			calculatedCost = (usage / totalUsage) * accountFee
		}
		calculatedCost += serverFee / 12.0 / float64(userCount)

		_, err = db.Exec(
			`INSERT INTO expense_usages (expense_id, user_id, usage, calculated_cost) VALUES (?, ?, ?, ?)`,
			expenseID, userID, usage, calculatedCost,
		)
		if err != nil {
			return 0, err
		}
	}

	return expenseID, nil
}

// 获取所有费用记录
func getAllExpenseRecords() ([]ExpenseRecord, error) {
	rows, err := db.Query(`SELECT id, start_date, end_date, account_fee, server_fee, created_at
		FROM expense_records ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []ExpenseRecord
	for rows.Next() {
		var r ExpenseRecord
		rows.Scan(&r.ID, &r.StartDate, &r.EndDate, &r.AccountFee, &r.ServerFee, &r.CreatedAt)
		records = append(records, r)
	}
	return records, nil
}

// 获取费用记录详情
func getExpenseRecordByID(id int) (*ExpenseRecord, error) {
	r := &ExpenseRecord{}
	err := db.QueryRow(
		`SELECT id, start_date, end_date, account_fee, server_fee, created_at FROM expense_records WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.StartDate, &r.EndDate, &r.AccountFee, &r.ServerFee, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// 获取费用记录的用户使用量
func getExpenseUsages(expenseID int) ([]ExpenseUsage, error) {
	rows, err := db.Query(`
		SELECT eu.id, eu.expense_id, eu.user_id, u.username, u.display_name, eu.usage, eu.calculated_cost
		FROM expense_usages eu
		LEFT JOIN users u ON eu.user_id = u.id
		WHERE eu.expense_id = ?
		ORDER BY eu.calculated_cost DESC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []ExpenseUsage
	for rows.Next() {
		var eu ExpenseUsage
		var username, displayName sql.NullString
		rows.Scan(&eu.ID, &eu.ExpenseID, &eu.UserID, &username, &displayName, &eu.Usage, &eu.CalculatedCost)
		if username.Valid {
			eu.Username = username.String
		} else {
			eu.Username = "已删除用户"
		}
		if displayName.Valid {
			eu.DisplayName = displayName.String
		} else {
			eu.DisplayName = "已删除用户"
		}
		usages = append(usages, eu)
	}
	return usages, nil
}

// 删除费用记录
func deleteExpenseRecord(id int) error {
	// 先删除使用量记录
	_, err := db.Exec("DELETE FROM expense_usages WHERE expense_id = ?", id)
	if err != nil {
		return err
	}
	// 再删除费用记录
	_, err = db.Exec("DELETE FROM expense_records WHERE id = ?", id)
	return err
}
