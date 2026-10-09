package middleware

import (
	"net/http"

	"dainxor/atv/configs"
	"dainxor/atv/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const ActiveProcessContextKey = "active_process_id"

func RequireActiveProcess() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("X-Process-ID")
		id, err := bson.ObjectIDFromHex(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "select a valid active process"})
			return
		}
		result := configs.DB.FindOne(bson.D{{Key: "_id", Value: id}}, models.ProcessDB{})
		if result.IsErr() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "process is unavailable"})
			return
		}
		process := result.Value().(models.ProcessDB)
		if !process.Active {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "process is disabled"})
			return
		}
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if user.Role != models.RoleAdmin {
			companionResult := configs.DB.FindOne(bson.D{{Key: "user_id", Value: user.ID}}, models.CompanionDB{})
			assigned := false
			if companionResult.IsOk() {
				companion := models.InterfaceTo[models.CompanionDB](companionResult.Value())
				for _, processID := range companion.IDProcess {
					if processID == id {
						assigned = true
						break
					}
				}
			}
			if !assigned {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "you are not assigned to this process"})
				return
			}
		}
		c.Set(ActiveProcessContextKey, id)
		c.Next()
	}
}

func ActiveProcessID(c *gin.Context) (bson.ObjectID, bool) {
	value, ok := c.Get(ActiveProcessContextKey)
	if !ok {
		return bson.NilObjectID, false
	}
	id, ok := value.(bson.ObjectID)
	return id, ok
}
