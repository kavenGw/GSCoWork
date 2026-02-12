package main

import "time"

type User struct {
	ID          int
	Username    string
	Password    string
	DisplayName string
	IsAdmin     bool
	CreatedAt   time.Time
}

type Schedule struct {
	ID     int
	UserID int
	Date   string // YYYY-MM-DD
	Status int    // 1=默认 2=休息 3=🐮🐴
}

const (
	StatusDefault = 1
	StatusRest    = 2
	StatusFire    = 3
)

// ExpenseRecord 费用记录
type ExpenseRecord struct {
	ID         int
	StartDate  string  // YYYY-MM-DD
	EndDate    string  // YYYY-MM-DD
	AccountFee float64 // 账户费用
	ServerFee  float64 // 服务器费用（年费）
	CreatedAt  time.Time
}

// ExpenseUsage 用户使用量记录
type ExpenseUsage struct {
	ID             int
	ExpenseID      int
	UserID         int
	Username       string
	DisplayName    string
	Usage          float64 // 使用量
	CalculatedCost float64 // 计算出的费用
}

// 默认费用配置
const (
	DefaultAccountFee = 550.0
	DefaultServerFee  = 99.0
)
