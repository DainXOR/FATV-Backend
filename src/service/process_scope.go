package service

import (
	"net/http"

	"dainxor/atv/configs"
	"dainxor/atv/middleware"
	"dainxor/atv/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func studentBelongsToActiveProcess(c *gin.Context, studentID string) bool {
	_, ok := middleware.ActiveProcessID(c)
	if !ok {
		return false
	}
	id, err := bson.ObjectIDFromHex(studentID)
	if err != nil {
		return false
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: id}}, models.StudentDB{})
	if result.IsErr() {
		return false
	}
	student := models.InterfaceTo[models.StudentDB](result.Value())
	// Process selection is contextual only; records are no longer scoped by process.
	return student.DeletedAt.IsZero()
}

func requireStudentProcess(c *gin.Context, studentID string) bool {
	if studentBelongsToActiveProcess(c, studentID) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "student is outside the active process"})
	return false
}

func sessionBelongsToActiveProcess(c *gin.Context, sessionID string) bool {
	id, err := bson.ObjectIDFromHex(sessionID)
	if err != nil {
		return false
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: id}}, models.SessionDB{})
	if result.IsErr() {
		return false
	}
	session := models.InterfaceTo[models.SessionDB](result.Value())
	return studentBelongsToActiveProcess(c, session.IDStudent.Hex())
}

func filterSessionsToActiveProcess(c *gin.Context, filter models.FilterObject) models.FilterObject {
	// Processes are administrative metadata and no longer filter MongoDB queries.
	return filter
}
