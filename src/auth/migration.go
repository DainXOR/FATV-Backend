package auth

import (
	"errors"
	"strings"
	"time"

	"dainxor/atv/configs"
	"dainxor/atv/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type companionAccountPatch struct {
	UserID    models.DBID       `bson:"user_id,omitempty"`
	UpdatedAt models.DBDateTime `bson:"updated_at,omitempty"`
}

type userSchemaMigration struct {
	ID        string    `bson:"_id"`
	AppliedAt time.Time `bson:"applied_at"`
}

func (userSchemaMigration) TableName() string { return "schema_migrations" }
func (m userSchemaMigration) IsZero() bool    { return m.ID == "" }

func (companionAccountPatch) TableName() string { return "companions" }
func (companionAccountPatch) IsZero() bool      { return false }

func migrateLegacyAccounts() error {
	const migrationID = "2026-09-30-users-companions-v1"
	marker := configs.DB.FindOne(bson.D{{Key: "_id", Value: migrationID}}, userSchemaMigration{})
	if marker.IsOk() {
		return nil
	}
	if !errors.Is(marker.Error(), configs.DBErr.NotFound()) {
		return marker.Error()
	}

	legacyResult := configs.DB.FindAll(bson.D{}, models.LegacyAuthUserDB{})
	if legacyResult.IsErr() && !errors.Is(legacyResult.Error(), configs.DBErr.NotFound()) {
		return legacyResult.Error()
	}
	if legacyResult.IsOk() {
		for _, item := range legacyResult.Value() {
			legacy := models.InterfaceTo[models.LegacyAuthUserDB](item)
			old := models.UserDB{
				ID: legacy.ID, Name: legacy.Name, PhoneNumber: legacy.PhoneNumber, Email: legacy.Email,
				PasswordHash: legacy.PasswordHash, Role: legacy.Role, Status: legacy.Status,
				EmailVerifiedAt: legacy.EmailVerifiedAt, CreatedAt: legacy.CreatedAt, UpdatedAt: legacy.UpdatedAt,
				DeletedAt: legacy.DeletedAt, CreatedBy: legacy.CreatedBy, LastLoginAt: legacy.LastLoginAt,
			}
			existing := configs.DB.FindOne(bson.D{{Key: "_id", Value: old.ID}}, models.UserDB{})
			if existing.IsOk() {
				continue
			}
			if existing.Error() != nil && !errors.Is(existing.Error(), configs.DBErr.NotFound()) {
				return existing.Error()
			}
			if _, err := configs.DB.InsertOne(&old).GetRaw(); err != nil {
				return err
			}
		}
	}
	companionsResult := configs.DB.FindAll(bson.D{}, models.LegacyCompanionDB{})
	if companionsResult.IsErr() && !errors.Is(companionsResult.Error(), configs.DBErr.NotFound()) {
		return companionsResult.Error()
	}
	if companionsResult.IsOk() {
		for _, item := range companionsResult.Value() {
			companion := models.InterfaceTo[models.LegacyCompanionDB](item)
			if !companion.UserID.IsZero() {
				if err := configs.DB.UnsetFields(bson.D{{Key: "_id", Value: companion.ID}}, models.CompanionDB{}, "number_id", "first_name", "last_name", "email", "phone_number"); err != nil {
					return err
				}
				continue
			}
			email := strings.TrimSpace(companion.Email)
			if email == "" {
				email = strings.TrimSpace(companion.InstitutionEmail)
			}
			if email == "" {
				continue
			}
			userResult := configs.DB.FindOne(bson.D{{Key: "email", Value: strings.ToLower(email)}}, models.UserDB{})
			var user models.UserDB
			if userResult.IsOk() {
				user = models.InterfaceTo[models.UserDB](userResult.Value())
			} else if errors.Is(userResult.Error(), configs.DBErr.NotFound()) {
				now := time.Now().UTC()
				user = models.UserDB{
					Name:        strings.TrimSpace(companion.FirstName + " " + companion.LastName),
					PhoneNumber: companion.PhoneNumber, Email: strings.ToLower(email),
					Role: models.RoleStaff, Status: models.AccountPending,
					CreatedAt: now, UpdatedAt: now,
				}
				inserted := configs.DB.InsertOne(&user)
				if inserted.IsErr() {
					return inserted.Error()
				}
				user.ID = inserted.Value()
			} else {
				return userResult.Error()
			}
			if user.Name == "" || user.PhoneNumber == "" {
				profilePatch := models.UserPatch{UpdatedAt: time.Now().UTC()}
				if user.Name == "" {
					profilePatch.Name = strings.TrimSpace(companion.FirstName + " " + companion.LastName)
				}
				if user.PhoneNumber == "" {
					profilePatch.PhoneNumber = companion.PhoneNumber
				}
				if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: user.ID}}, profilePatch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
					return err
				}
			}
			patch := companionAccountPatch{UserID: user.ID, UpdatedAt: models.Time.Now()}
			if err := configs.DB.PatchOne(bson.D{{Key: "_id", Value: companion.ID}}, patch); err != nil && !errors.Is(err, configs.DBErr.NotModified()) {
				return err
			}
			if err := configs.DB.UnsetFields(bson.D{{Key: "_id", Value: companion.ID}}, models.CompanionDB{}, "number_id", "first_name", "last_name", "email", "phone_number"); err != nil {
				return err
			}
		}
	}
	applied := userSchemaMigration{ID: migrationID, AppliedAt: time.Now().UTC()}
	if inserted := configs.DB.InsertOne(applied); inserted.IsErr() {
		return inserted.Error()
	}
	return nil
}
