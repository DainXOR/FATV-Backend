package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dainxor/atv/auth"
	"dainxor/atv/configs"
	"dainxor/atv/logger"
	"dainxor/atv/models"

	"github.com/gin-gonic/gin"
)

const (
	SessionCookieName = "fatv_session"
	CSRFCookieName    = "fatv_csrf"
)

type rateEntry struct {
	start time.Time
	count int
}

var authRateMu sync.Mutex
var authRate = map[string]rateEntry{}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if isPublicPath(path) {
			if isRateLimitedPath(path) && rateLimited(c.ClientIP()) {
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
				return
			}
			c.Next()
			return
		}
		cookie, err := c.Cookie(SessionCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		user, session, err := auth.Default.Authenticate(cookie)
		if err != nil {
			ClearSessionCookies(c)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session is invalid or expired"})
			return
		}
		if unsafeMethod(c.Request.Method) {
			cookieCSRF, cookieErr := c.Cookie(CSRFCookieName)
			headerCSRF := c.GetHeader("X-CSRF-Token")
			if cookieErr != nil || headerCSRF == "" ||
				subtle.ConstantTimeCompare([]byte(cookieCSRF), []byte(headerCSRF)) != 1 {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF validation failed"})
				return
			}
		}
		c.Set("auth_user", user)
		c.Set("auth_session", session)
		c.Next()
	}
}

func isPublicPath(path string) bool {
	versioned := "/api/v" + uintToString(configs.App.RoutesVersion()) + "/auth"
	if path == "/" || path == "/api/info" || strings.HasPrefix(path, "/api/info/") {
		return true
	}
	if strings.HasPrefix(path, versioned+"/login") ||
		strings.HasPrefix(path, versioned+"/setup") ||
		strings.HasPrefix(path, versioned+"/recovery/request") ||
		strings.HasPrefix(path, versioned+"/recovery/complete") ||
		strings.HasPrefix(path, "/api/v"+uintToString(configs.App.RoutesVersion())+"/public/forms/") {
		return true
	}
	return false
}

func isRateLimitedPath(path string) bool {
	return strings.Contains(path, "/auth/login") ||
		strings.Contains(path, "/auth/setup") ||
		strings.Contains(path, "/auth/recovery/")
}

func rateLimited(ip string) bool {
	now := time.Now()
	authRateMu.Lock()
	defer authRateMu.Unlock()
	entry := authRate[ip]
	if now.Sub(entry.start) >= time.Minute || entry.start.IsZero() {
		entry = rateEntry{start: now}
	}
	entry.count++
	authRate[ip] = entry
	if len(authRate) > 5000 {
		for key, value := range authRate {
			if now.Sub(value.start) > 5*time.Minute {
				delete(authRate, key)
			}
		}
	}
	return entry.count > 20
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func CurrentUser(c *gin.Context) (models.AuthUserDB, bool) {
	value, ok := c.Get("auth_user")
	if !ok {
		return models.AuthUserDB{}, false
	}
	user, ok := value.(models.AuthUserDB)
	return user, ok
}

func RequireRole(role models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if user.Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

func uintToString(value uint32) string {
	if value == 0 {
		return "0"
	}
	var buffer [10]byte
	i := len(buffer)
	for value > 0 {
		i--
		buffer[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[i:])
}

func SetSessionCookies(c *gin.Context, token, csrf string, expires time.Time) {
	secure := auth.SessionCookieSecure()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, token, 0, "/", "", secure, true)
	c.SetCookie(CSRFCookieName, csrf, 0, "/", "", secure, false)
	c.Header("Cache-Control", "no-store")
}

func ClearSessionCookies(c *gin.Context) {
	secure := auth.SessionCookieSecure()
	c.SetCookie(SessionCookieName, "", -1, "/", "", secure, true)
	c.SetCookie(CSRFCookieName, "", -1, "/", "", secure, false)
}

func ValidateFrontendOrigin(c *gin.Context) bool {
	origin := c.GetHeader("Origin")
	if origin == "" {
		return true
	}
	if origin == "http://localhost:3000" && !strings.EqualFold(os.Getenv("APP_ENV"), "prod") {
		return true
	}
	configured := strings.TrimRight(os.Getenv("FRONTEND_ORIGIN"), "/")
	return configured != "" && origin == configured
}

func LogAuthFailure(event string) {
	logger.Warning("Authentication request rejected:", event)
}
