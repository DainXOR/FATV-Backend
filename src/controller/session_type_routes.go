package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func SessionTypesRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/session-type", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/session-types", rv)

	sessionTypeRouterOld := router.Group(beforeRoute)
	{
		sessionTypeRouterOld.POST("", service.SessionType.Create)

		sessionTypeRouterOld.GET("/:id", service.SessionType.GetByID)
		sessionTypeRouterOld.GET("/all", service.SessionType.GetAll)

		sessionTypeRouterOld.PUT("/:id", service.SessionType.UpdateByID)

		sessionTypeRouterOld.PATCH("/:id", service.SessionType.PatchByID)

		sessionTypeRouterOld.DELETE("/:id", service.SessionType.DeleteByID)
	}

	//router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
	//	ctx.JSON(types.Http.C300().PermanentRedirect(),
	//		types.EmptyResponse(
	//			logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
	//		),
	//	)
	//	ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
	//})

	sessionTypeRouter := router.Group(lastRoute)
	{
		sessionTypeRouter.POST("", service.SessionType.Create)

		sessionTypeRouter.GET("/:id", service.SessionType.GetByID)
		sessionTypeRouter.GET("/all", service.SessionType.GetAll)

		sessionTypeRouter.PUT("/:id", service.SessionType.UpdateByID)

		sessionTypeRouter.PATCH("/:id", service.SessionType.PatchByID)

		sessionTypeRouter.DELETE("/:id", service.SessionType.DeleteByID)
	}
}
