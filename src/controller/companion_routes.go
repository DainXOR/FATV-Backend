package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func CompanionsRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/companion", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/companions", rv)

	companionRouterOld := router.Group(beforeRoute)
	{
		companionRouterOld.GET("/:id", service.Companion.GetByID)
		companionRouterOld.GET("/all", service.Companion.GetAll)

		companionRouterOld.POST("", service.Companion.Create)

		companionRouterOld.PUT("/:id", service.Companion.UpdateByID)

		companionRouterOld.PATCH("/:id", service.Companion.PatchByID)

		companionRouterOld.DELETE("/:id", service.Companion.DeleteByID)
		//companionRouterOld.DELETE("/permanent-delete/:id/:confirm", controller.Student.ForceDeleteByID)

	}
	//router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
	//	ctx.JSON(types.Http.C300().PermanentRedirect(),
	//		types.EmptyResponse(
	//			logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
	//		),
	//	)
	//	ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
	//})

	companionRouter := router.Group(lastRoute)
	{
		companionRouter.GET("/:id", service.Companion.GetByID)
		companionRouter.GET("/all", service.Companion.GetAll)

		companionRouter.POST("", service.Companion.Create)

		companionRouter.PUT("/:id", service.Companion.UpdateByID)

		companionRouter.PATCH("/:id", service.Companion.PatchByID)

		companionRouter.DELETE("/:id", service.Companion.DeleteByID)
		//companionRouter.DELETE("/permanent-delete/:id/:confirm", controller.Student.ForceDeleteByID)

	}
}
