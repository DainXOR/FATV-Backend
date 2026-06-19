package middleware

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	jcors "github.com/itsjamie/gin-cors"
)

func CORS() gin.HandlerFunc {
	return corsLib()
}

func corsOwn() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		// c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func corsLib() gin.HandlerFunc {
	allowedOrigins := map[string]bool{
		"http://localhost:3000": true,
	}

	return cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders: []string{"Origin", "Content-Length", "Content-Type", "Authorization", "X-Request-ID", "Access-Control-Allow-Origin"},
		// AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool { return allowedOrigins[origin] },
	})
}

func corsJamie() gin.HandlerFunc {
	return jcors.Middleware(jcors.Config{
		Origins:        "http://localhost:3000, https://fuzzy-fiesta-g6xqxp4w6vw296v-3000.app.github.dev/",
		Methods:        "GET, PUT, POST, PATCH, DELETE",
		RequestHeaders: "Origin, Content-Type, Content-Length",
		// Credentials:     true,
		ValidateHeaders: false,
	})
}
