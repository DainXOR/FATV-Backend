package controller

import (
	"errors"
	"net/http"
	"strconv"

	"dainxor/atv/auth"
	"dainxor/atv/configs"
	"dainxor/atv/logger"
	"dainxor/atv/middleware"
	"dainxor/atv/models"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type loginBody struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type setupBody struct {
	Email    string `json:"email" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type recoveryRequestBody struct {
	Email string `json:"email" binding:"required"`
}
type accountCreateBody struct {
	Email string          `json:"email" binding:"required"`
	Role  models.UserRole `json:"role"`
}
type statusBody struct {
	Status models.AccountStatus `json:"status" binding:"required"`
}
type roleBody struct {
	Role models.UserRole `json:"role" binding:"required"`
}
type settingsBody struct {
	IdleTimeoutMinutes          int  `json:"idle_timeout_minutes" binding:"required"`
	AbsoluteLifetimeMinutes     int  `json:"absolute_session_lifetime_minutes" binding:"required"`
	SelfServiceRecoveryEnabled  bool `json:"self_service_recovery_enabled"`
	SetupCodeLifetimeMinutes    int  `json:"setup_code_lifetime_minutes" binding:"required"`
	RecoveryCodeLifetimeMinutes int  `json:"recovery_code_lifetime_minutes" binding:"required"`
}

func AuthRoutes(router *gin.Engine) {
	base := "/api/v" + routeVersionString() + "/auth"
	r := router.Group(base)
	r.POST("/login", login)
	r.POST("/setup", completeSetup)
	r.POST("/recovery/request", requestRecovery)
	r.POST("/recovery/complete", completeRecovery)
	r.GET("/me", me)
	r.POST("/logout", logout)

	admin := r.Group("")
	admin.Use(middleware.RequireRole(models.RoleAdmin))
	{
		admin.POST("/accounts", createAccount)
		admin.GET("/accounts", listAccounts)
		admin.PATCH("/accounts/:id/status", updateAccountStatus)
		admin.PATCH("/accounts/:id/role", updateAccountRole)
		admin.POST("/accounts/:id/setup", adminResendSetup)
		admin.POST("/accounts/:id/recovery", adminRecovery)
		admin.GET("/settings", getSettings)
		admin.PATCH("/settings", updateSettings)
	}
}

func login(c *gin.Context) {
	if !middleware.ValidateFrontendOrigin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
		return
	}
	var body loginBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}
	result, err := auth.Default.Login(body.Email, body.Password)
	if err != nil {
		logger.Warning("Login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	middleware.SetSessionCookies(c, result.SessionToken, result.CSRFToken, result.ExpiresAt)
	c.JSON(http.StatusOK, gin.H{"user": auth.PublicUserFrom(result.User), "csrf_token": result.CSRFToken})
}

func completeSetup(c *gin.Context) {
	var body setupBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, code, and password are required"})
		return
	}
	if err := auth.Default.CompleteSetup(body.Email, body.Code, body.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "setup code is invalid or expired, or password is invalid"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Account setup completed. You can now sign in."})
}

func requestRecovery(c *gin.Context) {
	var body recoveryRequestBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusOK, gin.H{"message": "If the account is eligible, recovery instructions will be sent."})
		return
	}
	if err := auth.Default.RequestRecovery(body.Email); err != nil {
		logger.Warning("Password recovery email could not be sent:", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "If the account is eligible, recovery instructions will be sent."})
}

func completeRecovery(c *gin.Context) {
	var body setupBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email, code, and password are required"})
		return
	}
	if err := auth.Default.CompleteRecovery(body.Email, body.Code, body.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recovery code is invalid or expired, or password is invalid"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password changed. Please sign in again."})
}

func me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"user": auth.PublicUserFrom(user)})
}

func logout(c *gin.Context) {
	token, _ := c.Cookie(middleware.SessionCookieName)
	auth.Default.Logout(token)
	middleware.ClearSessionCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func createAccount(c *gin.Context) {
	var body accountCreateBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}
	if body.Role == "" {
		body.Role = models.RoleStaff
	}
	if !body.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin or staff"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	if err := auth.Default.CreateAccount(body.Email, body.Role, actor.ID); err != nil {
		if errors.Is(err, auth.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "an account with this email already exists"})
			return
		}
		logger.Warning("Account creation failed:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "account could not be created or setup email could not be sent"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "Account created; setup instructions sent by email."})
}

func listAccounts(c *gin.Context) {
	users, err := auth.Default.ListAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load accounts"})
		return
	}
	result := make([]auth.PublicUser, 0, len(users))
	for _, user := range users {
		result = append(result, auth.PublicUserFrom(user))
	}
	c.JSON(http.StatusOK, gin.H{"accounts": result})
}

func updateAccountStatus(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	var body statusBody
	if c.ShouldBindJSON(&body) != nil || !body.Status.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account status"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	if err := auth.Default.UpdateAccountStatus(actor.ID, id, body.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func updateAccountRole(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	var body roleBody
	if c.ShouldBindJSON(&body) != nil || !body.Role.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be admin or staff"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	if err := auth.Default.UpdateAccountRole(actor.ID, id, body.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func adminResendSetup(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	if err := auth.Default.AdminResendSetup(actor.ID, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "setup instructions could not be sent"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Setup instructions sent."})
}

func adminRecovery(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	if err := auth.Default.AdminIssueRecovery(actor.ID, id); err != nil {
		logger.Warning("Admin recovery request failed:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "recovery instructions could not be sent"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Recovery instructions sent."})
}

func getSettings(c *gin.Context) {
	settings, err := auth.Default.Settings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load security settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func updateSettings(c *gin.Context) {
	var body settingsBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid security settings"})
		return
	}
	actor, _ := middleware.CurrentUser(c)
	err := auth.Default.UpdateSettings(actor.ID, models.AuthSettingsDB{
		IdleTimeoutMinutes:          body.IdleTimeoutMinutes,
		AbsoluteLifetimeMinutes:     body.AbsoluteLifetimeMinutes,
		SelfServiceRecoveryEnabled:  body.SelfServiceRecoveryEnabled,
		SetupCodeLifetimeMinutes:    body.SetupCodeLifetimeMinutes,
		RecoveryCodeLifetimeMinutes: body.RecoveryCodeLifetimeMinutes,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Security settings updated"})
}

func routeVersionString() string {
	return strconv.FormatUint(uint64(configs.App.RoutesVersion()), 10)
}
