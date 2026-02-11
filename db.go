package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
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
