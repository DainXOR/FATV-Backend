package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func UniversitiesRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/university", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/universities", rv)

	universityRouterOld := router.Group(beforeRoute)
	{
		universityRouterOld.POST("", service.University.Create)

		universityRouterOld.GET("/:id", service.University.GetByID)
		universityRouterOld.GET("/all", service.University.GetAll)

		universityRouterOld.PUT("/:id", service.University.UpdateByID)

		universityRouterOld.PATCH("/:id", service.University.PatchByID)

		universityRouterOld.DELETE("/:id", service.University.DeleteByID)
	}

	//router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
	//	ctx.JSON(types.Http.C300().PermanentRedirect(),
	//		types.EmptyResponse(
	//			logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
	//		),
	//	)
	//	ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
	//})

	universityRouter := router.Group(lastRoute)
	{
		universityRouter.POST("/", service.University.Create)

		universityRouter.GET("/:id", service.University.GetByID)
		universityRouter.GET("/all", service.University.GetAll)

		universityRouter.PUT("/:id", service.University.UpdateByID)

		universityRouter.PATCH("/:id", service.University.PatchByID)

		universityRouter.DELETE("/:id", service.University.DeleteByID)
	}
}
