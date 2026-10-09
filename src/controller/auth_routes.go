package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	Name             string          `json:"name"`
	Email            string          `json:"email" binding:"required"`
	PhoneNumber      string          `json:"phone_number"`
	Role             models.UserRole `json:"role"`
	InstitutionEmail string          `json:"institution_email"`
	SpecialityID     string          `json:"speciality_id"`
	IDProcess        []string        `json:"id_process"`
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
	var specialityID models.DBID
	var processIDs []models.DBID
	if body.Role == models.RoleStaff {
		if body.Name == "" || body.InstitutionEmail == "" || body.SpecialityID == "" || len(body.IDProcess) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "staff accounts require name, institution_email, speciality_id, and at least one process"})
			return
		}
		parsedSpeciality, parseErr := models.ID.ToDB(body.SpecialityID)
		if parseErr != nil || configs.DB.FindOne(bson.D{{Key: "_id", Value: parsedSpeciality}}, models.SpecialityDB{}).IsErr() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid speciality_id"})
			return
		}
		specialityID = parsedSpeciality
		seen := map[string]bool{}
		for _, raw := range body.IDProcess {
			pid, parseErr := bson.ObjectIDFromHex(raw)
			if parseErr != nil || seen[raw] {
				if parseErr != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid process id"})
					return
				}
				continue
			}
			processResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: pid}}, models.ProcessDB{})
			if processResult.IsErr() || !models.InterfaceTo[models.ProcessDB](processResult.Value()).Active {
				c.JSON(http.StatusBadRequest, gin.H{"error": "all assigned processes must exist and be active"})
				return
			}
			seen[raw] = true
			processIDs = append(processIDs, pid)
		}
		if len(processIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one process is required"})
			return
		}
	}
	if err := auth.Default.CreateAccountWithProfile(body.Email, body.Name, body.PhoneNumber, body.Role, actor.ID); err != nil {
		if errors.Is(err, auth.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "an account with this email already exists"})
			return
		}
		logger.Warning("Account creation failed:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "account could not be created or setup email could not be sent"})
		return
	}
	if body.Role == models.RoleStaff {
		userResult := configs.DB.FindOne(bson.D{{Key: "email", Value: strings.ToLower(strings.TrimSpace(body.Email))}}, models.UserDB{})
		if userResult.IsErr() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "account created but user profile could not be loaded"})
			return
		}
		user := models.InterfaceTo[models.UserDB](userResult.Value())
		companion := models.CompanionDB{UserID: user.ID, IDProcess: processIDs, InstitutionEmail: strings.TrimSpace(body.InstitutionEmail),
			IDSpeciality: specialityID, CreatedAt: models.Time.Now(), UpdatedAt: models.Time.Now(), DeletedAt: models.Time.Zero()}
		inserted := configs.DB.InsertOne(&companion)
		if inserted.IsErr() {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "account created but companion profile could not be saved"})
			return
		}
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
		public := auth.PublicUserFrom(user)
		if user.Role == models.RoleStaff {
			companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: user.ID}}, models.CompanionDB{})
			if companionResult.IsOk() {
				companion := models.InterfaceTo[models.CompanionDB](companionResult.Value())
				public.CompanionID = companion.ID.Hex()
				public.InstitutionEmail = companion.InstitutionEmail
				public.SpecialityID = companion.IDSpeciality.Hex()
				public.IDProcess = make([]string, 0, len(companion.IDProcess))
				for _, processID := range companion.IDProcess {
					public.IDProcess = append(public.IDProcess, processID.Hex())
				}
			}
		}
		result = append(result, public)
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
