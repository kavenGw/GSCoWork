package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// Session 有效期
	SessionDuration         = 24 * time.Hour      // 普通 session：1 天
	RememberMeDuration      = 30 * 24 * time.Hour // 记住我：30 天
	SessionCleanupInterval  = 1 * time.Hour       // 清理过期 session 间隔
)

type Session struct {
	UserID    int
	Username  string
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
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

// createSession 创建 session，rememberMe 为 true 时设置更长的有效期
func createSession(w http.ResponseWriter, user *User, rememberMe bool) {
	sid := generateSessionID()

	var duration time.Duration
	if rememberMe {
		duration = RememberMeDuration
	} else {
		duration = SessionDuration
	}

	now := time.Now()
	expiresAt := now.Add(duration)

	sess := &Session{
		UserID:    user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}

	// 保存到内存
	sessMu.Lock()
	sessions[sid] = sess
	sessMu.Unlock()

	// 如果记住我，持久化到数据库
	if rememberMe {
		saveSessionToDB(sid, user.ID, expiresAt.Format(time.RFC3339))
	}

	// 设置 cookie
	cookie := &http.Cookie{
		Name:     "session",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	if rememberMe {
		cookie.MaxAge = int(duration.Seconds())
	}

	http.SetCookie(w, cookie)
}

func getSession(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}

	sid := cookie.Value

	// 先从内存查找
	sessMu.RLock()
	sess, exists := sessions[sid]
	sessMu.RUnlock()

	if exists {
		// 检查是否过期
		if time.Now().After(sess.ExpiresAt) {
			// 已过期，清理
			sessMu.Lock()
			delete(sessions, sid)
			sessMu.Unlock()
			deleteSessionFromDB(sid)
			return nil
		}
		return sess
	}

	// 内存中没有，尝试从数据库恢复（服务重启后）
	userID, expiresAtStr, err := getSessionFromDB(sid)
	if err != nil {
		return nil
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		deleteSessionFromDB(sid)
		return nil
	}

	// 检查是否过期
	if time.Now().After(expiresAt) {
		deleteSessionFromDB(sid)
		return nil
	}

	// 从数据库获取用户信息
	user, err := getUserByID(userID)
	if err != nil {
		deleteSessionFromDB(sid)
		return nil
	}

	// 恢复到内存
	sess = &Session{
		UserID:    user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	sessMu.Lock()
	sessions[sid] = sess
	sessMu.Unlock()

	return sess
}

func destroySession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return
	}

	sid := cookie.Value

	// 从内存删除
	sessMu.Lock()
	delete(sessions, sid)
	sessMu.Unlock()

	// 从数据库删除
	deleteSessionFromDB(sid)

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

// startSessionCleanup 启动定期清理过期 session 的后台任务
func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(SessionCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			// 清理数据库中过期的 session
			cleanExpiredSessions()

			// 清理内存中过期的 session
			now := time.Now()
			sessMu.Lock()
			for sid, sess := range sessions {
				if now.After(sess.ExpiresAt) {
					delete(sessions, sid)
				}
			}
			sessMu.Unlock()
		}
	}()
}
