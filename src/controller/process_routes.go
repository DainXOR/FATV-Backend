package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/middleware"
	"dainxor/atv/models"
	"errors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func ProcessRoutes(router *gin.Engine) {
	base := "/api/v" + routeVersionString()
	r := router.Group(base + "/processes")
	r.GET("", listProcesses)
	adminProcesses := r.Group("")
	adminProcesses.Use(middleware.RequireRole(models.RoleAdmin))
	adminProcesses.POST("", createProcess)
	adminProcesses.PATCH("/:id", updateProcess)

	admin := router.Group(base)
	admin.Use(middleware.RequireRole(models.RoleAdmin))
	admin.GET("/admin/students", listAllStudentsForAdmin)
	admin.PATCH("/accounts/:id/processes", assignStaffProcesses)
	admin.PATCH("/students/:id/process", assignStudentProcess)
}

func listProcesses(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	result := configs.DB.FindAll(bson.D{}, models.ProcessDB{})
	if result.IsErr() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load processes"})
		return
	}
	assigned := map[string]bool{}
	if user.Role == models.RoleStaff {
		companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: user.ID}}, models.CompanionDB{})
		if companionResult.IsOk() {
			companion := models.InterfaceTo[models.CompanionDB](companionResult.Value())
			for _, id := range companion.IDProcess {
				assigned[id.Hex()] = true
			}
		}
	}
	items := make([]models.ProcessDB, 0)
	for _, raw := range result.Value() {
		p := models.InterfaceTo[models.ProcessDB](raw)
		if user.Role == models.RoleAdmin || (assigned[p.ID.Hex()] && p.Active) {
			items = append(items, p)
		}
	}
	c.JSON(http.StatusOK, gin.H{"processes": items})
}

func createProcess(c *gin.Context) {
	var body models.ProcessCreate
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a process name is required"})
		return
	}
	active := true
	if body.Active != nil {
		active = *body.Active
	}
	now := time.Now().UTC()
	p := models.ProcessDB{Name: strings.TrimSpace(body.Name), Active: active, CreatedAt: now, UpdatedAt: now}
	result := configs.DB.InsertOne(p)
	if result.IsErr() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create process"})
		return
	}
	p.ID = result.Value()
	c.JSON(http.StatusCreated, gin.H{"process": p})
}

func updateProcess(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid process id"})
		return
	}
	var body models.ProcessUpdate
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a process name and active status are required"})
		return
	}
	patch := models.ProcessPatch{Name: strings.TrimSpace(body.Name), Active: &body.Active, UpdatedAt: time.Now().UTC()}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: id}}, patch); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "process not found or unchanged"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "process updated"})
}

func assignStaffProcesses(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}
	var body models.StaffProcessAssignment
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_process is required"})
		return
	}
	userResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: id}}, models.UserDB{})
	if userResult.IsErr() {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	user := userResult.Value().(models.UserDB)
	if user.Role != models.RoleStaff {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only staff accounts can be assigned to processes"})
		return
	}
	ids := make([]models.DBID, 0, len(body.IDProcess))
	seen := map[string]bool{}
	for _, raw := range body.IDProcess {
		pid, parseErr := bson.ObjectIDFromHex(raw)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid process id"})
			return
		}
		if seen[raw] {
			continue
		}
		seen[raw] = true
		p := configs.DB.FindOne(bson.D{{Key: "_id", Value: pid}}, models.ProcessDB{})
		if p.IsErr() || !models.InterfaceTo[models.ProcessDB](p.Value()).Active {
			c.JSON(http.StatusBadRequest, gin.H{"error": "all assigned processes must exist and be active"})
			return
		}
		ids = append(ids, pid)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "each companion must have at least one process"})
		return
	}
	companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: id}}, models.CompanionDB{})
	if companionResult.IsErr() {
		c.JSON(http.StatusNotFound, gin.H{"error": "companion profile not found"})
		return
	}
	companion := models.InterfaceTo[models.CompanionDB](companionResult.Value())
	companionPatch := models.CompanionProcessIDsPatch{IDProcess: ids, UpdatedAt: models.Time.Now()}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: companion.ID}}, companionPatch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update companion process assignments"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "staff process assignments updated"})
}

func assignStudentProcess(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid student id"})
		return
	}
	var body models.StudentProcessAssignment
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "process_id is required"})
		return
	}
	pid, err := bson.ObjectIDFromHex(body.ProcessID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid process id"})
		return
	}
	p := configs.DB.FindOne(bson.D{{Key: "_id", Value: pid}}, models.ProcessDB{})
	if p.IsErr() || !models.InterfaceTo[models.ProcessDB](p.Value()).Active {
		c.JSON(http.StatusBadRequest, gin.H{"error": "an active process is required"})
		return
	}
	patch := models.StudentProcessPatch{ProcessID: pid, UpdatedAt: time.Now().UTC()}
	if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: id}}, patch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "student not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "student process updated"})
}

type processAssignmentLegacyUser struct {
	ID         models.DBID     `bson:"_id,omitempty"`
	Role       models.UserRole `bson:"role"`
	ProcessIDs []models.DBID   `bson:"process_ids,omitempty"`
}

func (processAssignmentLegacyUser) TableName() string { return "users" }
func (u processAssignmentLegacyUser) IsZero() bool    { return u.ID.IsZero() }

type processDataMigration struct {
	ID        string    `json:"id" bson:"_id"`
	AppliedAt time.Time `json:"applied_at" bson:"applied_at"`
}

func (processDataMigration) TableName() string { return "schema_migrations" }
func (m processDataMigration) IsZero() bool    { return m.ID == "" }

type formProcessTagsPatch struct {
	ProcessTags []string  `bson:"process_tags"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (formProcessTagsPatch) TableName() string { return "forms" }
func (formProcessTagsPatch) IsZero() bool      { return false }

func randomProcess(processes []models.ProcessDB) models.ProcessDB {
	return processes[rand.Intn(len(processes))]
}

func InitializeProcesses() error {
	result := configs.DB.FindAll(bson.D{}, models.ProcessDB{})
	var processes []models.ProcessDB
	if result.IsOk() {
		for _, item := range result.Value() {
			processes = append(processes, models.InterfaceTo[models.ProcessDB](item))
		}
	} else if result.Error() != configs.DBErr.NotFound() {
		return result.Error()
	}

	activeProcesses := make([]models.ProcessDB, 0, len(processes))
	for _, process := range processes {
		if process.Active {
			activeProcesses = append(activeProcesses, process)
		}
	}
	if len(activeProcesses) == 0 {
		now := time.Now().UTC()
		process := models.ProcessDB{Name: "Proceso general", Active: true, CreatedAt: now, UpdatedAt: now}
		inserted := configs.DB.InsertOne(process)
		if inserted.IsErr() {
			return inserted.Error()
		}
		process.ID = inserted.Value()
		activeProcesses = append(activeProcesses, process)
	}

	const migrationID = "2026-09-30-process-assignments-v3"
	migration := configs.DB.FindOne(bson.D{{Key: "_id", Value: migrationID}}, processDataMigration{})
	if migration.IsOk() {
		return nil
	}
	if !errors.Is(migration.Error(), configs.DBErr.NotFound()) {
		return migration.Error()
	}

	studentsResult := configs.DB.FindAll(bson.D{}, models.StudentDB{})
	if studentsResult.IsErr() && !errors.Is(studentsResult.Error(), configs.DBErr.NotFound()) {
		return studentsResult.Error()
	}
	if studentsResult.IsOk() {
		for _, raw := range studentsResult.Value() {
			student := models.InterfaceTo[models.StudentDB](raw)
			if student.ProcessID.IsZero() {
				chosen := randomProcess(activeProcesses)
				patch := models.StudentProcessPatch{ProcessID: chosen.ID, UpdatedAt: time.Now().UTC()}
				if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: student.ID}}, patch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
					return err
				}
			}
		}
	}

	accountsResult := configs.DB.FindAll(bson.D{}, processAssignmentLegacyUser{})
	if accountsResult.IsErr() && !errors.Is(accountsResult.Error(), configs.DBErr.NotFound()) {
		return accountsResult.Error()
	}
	if accountsResult.IsOk() {
		for _, raw := range accountsResult.Value() {
			account := models.InterfaceTo[processAssignmentLegacyUser](raw)
			if err := configs.DB.UnsetFields(bson.D{{Key: "_id", Value: account.ID}}, models.UserDB{}, "process_ids"); err != nil {
				return err
			}
			if account.Role != models.RoleStaff {
				continue
			}
			companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: account.ID}}, models.LegacyCompanionDB{})
			if companionResult.IsErr() {
				continue
			}
			companion := models.InterfaceTo[models.LegacyCompanionDB](companionResult.Value())
			ids := append([]models.DBID{}, companion.IDProcess...)
			if len(ids) == 0 && !companion.ProcessID.IsZero() {
				ids = append(ids, companion.ProcessID)
			}
			for _, id := range account.ProcessIDs {
				found := false
				for _, existingID := range ids {
					if existingID == id {
						found = true
						break
					}
				}
				if !found {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				ids = append(ids, randomProcess(activeProcesses).ID)
			}
			patch := models.CompanionProcessIDsPatch{IDProcess: ids, UpdatedAt: models.Time.Now()}
			if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: companion.ID}}, patch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
				return err
			}
			if err := configs.DB.UnsetFields(bson.D{{Key: "_id", Value: companion.ID}}, models.CompanionDB{}, "process_id"); err != nil {
				return err
			}
		}
	}
	legacyCompanions := configs.DB.FindAll(bson.D{}, models.LegacyCompanionDB{})
	if legacyCompanions.IsErr() && !errors.Is(legacyCompanions.Error(), configs.DBErr.NotFound()) {
		return legacyCompanions.Error()
	}
	if legacyCompanions.IsOk() {
		for _, raw := range legacyCompanions.Value() {
			companion := models.InterfaceTo[models.LegacyCompanionDB](raw)
			if len(companion.IDProcess) == 0 {
				ids := []models.DBID{}
				if !companion.ProcessID.IsZero() {
					ids = append(ids, companion.ProcessID)
				}
				if len(ids) == 0 {
					ids = append(ids, randomProcess(activeProcesses).ID)
				}
				if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: companion.ID}}, models.CompanionProcessIDsPatch{IDProcess: ids, UpdatedAt: models.Time.Now()}); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
					return err
				}
			}
			if err := configs.DB.UnsetFields(bson.D{{Key: "_id", Value: companion.ID}}, models.CompanionDB{}, "process_id", "number_id", "first_name", "last_name", "email", "phone_number"); err != nil {
				return err
			}
		}
	}
	if err := configs.DB.UnsetFields(bson.D{}, models.UserDB{}, "process_ids"); err != nil {
		return err
	}

	formsResult := configs.DB.FindAll(bson.D{}, models.FormDB{})
	if formsResult.IsErr() && !errors.Is(formsResult.Error(), configs.DBErr.NotFound()) {
		return formsResult.Error()
	}
	if formsResult.IsOk() {
		for _, raw := range formsResult.Value() {
			form := models.InterfaceTo[models.FormDB](raw)
			if len(form.ProcessTags) == 0 {
				chosen := randomProcess(activeProcesses)
				patch := formProcessTagsPatch{ProcessTags: []string{chosen.ID.Hex()}, UpdatedAt: time.Now().UTC()}
				if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: form.ID}}, patch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
					return err
				}
			}
		}
	}

	marker := processDataMigration{ID: migrationID, AppliedAt: time.Now().UTC()}
	if inserted := configs.DB.InsertOne(marker); inserted.IsErr() {
		return inserted.Error()
	}
	return nil
}

func listAllStudentsForAdmin(c *gin.Context) {
	result := configs.DB.FindAll(bson.D{{Key: "deleted_at", Value: models.Time.Zero()}}, models.StudentDB{})
	if result.IsErr() {
		c.JSON(http.StatusOK, gin.H{"students": []models.StudentResponse{}})
		return
	}
	students := make([]models.StudentResponse, 0, len(result.Value()))
	for _, item := range result.Value() {
		student := models.InterfaceTo[models.StudentDB](item)
		students = append(students, student.ToResponse())
	}
	c.JSON(http.StatusOK, gin.H{"students": students})
}
