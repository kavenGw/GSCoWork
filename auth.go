package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Session struct {
	UserID    int
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
}

var (
	sessions = make(map[string]*Session)
	sessMu   sync.RWMutex
)

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func createSession(w http.ResponseWriter, user *User) {
	sid := generateSessionID()
	sessMu.Lock()
	sessions[sid] = &Session{
		UserID:    user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		CreatedAt: time.Now(),
	}
	sessMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	sessMu.RLock()
	defer sessMu.RUnlock()
	return sessions[cookie.Value]
}

func destroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return
	}
	sessMu.Lock()
	delete(sessions, cookie.Value)
	sessMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func requireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if getSession(r) == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return requireLogin(func(w http.ResponseWriter, r *http.Request) {
		sess := getSession(r)
		if !sess.IsAdmin {
			http.Error(w, "无权访问", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func checkPassword(hashed, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
}
