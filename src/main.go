package main

import (
	"dainxor/atv/auth"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"

	"dainxor/atv/configs"
	"dainxor/atv/controller"
	"dainxor/atv/logger"
	"dainxor/atv/middleware"
)

//var envErr = godotenv.Load()

func init() {
	logger.SetVersion(configs.App.ApiVersion())
	if err := configs.DB.Start(); err != nil {
		logger.Fatal("Failed to connect to the database:", err)
	}
	if err := auth.Default.Initialize(); err != nil {
		logger.Fatal("Failed to initialize authentication:", err)
	}
	if err := auth.Default.BootstrapAdminFromEnv(); err != nil {
		logger.Fatal("Failed to bootstrap administrator:", err)
	}

	if !configs.WebHooks.IsReady() {
		logger.Warning("Webhook broker not ready at startup — will retry in background")
	}

	logger.Info("Env configurations loaded")
	logger.Debug("Starting server")
}

func main() {
	defer configs.DB.Close()
	defer configs.WebHooks.Close()
	defer logger.Close()

	router := gin.New()
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recovery()) // Middleware to recover from panics and logs a small trace
	router.Use(middleware.CORS())
	router.Use(middleware.AuthMiddleware())

	// Public authentication endpoints and protected account administration.
	controller.AuthRoutes(router)
	controller.FormAccessRoutes(router)

	// Root level routes
	controller.MainRoutes(router)

	// Versioned API routes
	controller.StudentsRoutes(router)
	controller.UniversitiesRoutes(router)
	controller.SpecialitiesRoutes(router)
	controller.CompanionsRoutes(router)
	controller.SessionTypesRoutes(router)
	controller.SessionsRoutes(router)
	controller.PrioritiesRoutes(router)
	controller.AlertsRoutes(router)
	controller.ContactReasonsRoutes(router)
	controller.VulnerabilityTypesRoutes(router)
	controller.FormsRoutes(router)

	// Api informative routes
	controller.TestRoutes(router) // Routes for testing purposes
	controller.InfoRoutes(router) // Routes for information about the API

	router.Run(configs.App.Address()) // listen and serve on 0.0.0.0:8080 (for windows ":8080")
}
