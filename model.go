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
	Status int    // 1=默认 2=休息 3=鸡血
}

const (
	StatusDefault = 1
	StatusRest    = 2
	StatusFire    = 3
)
