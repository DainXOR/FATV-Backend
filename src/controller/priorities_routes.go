package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func PrioritiesRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/priorities", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/priorities", rv)

	priorityRouterOld := router.Group(beforeRoute)
	{
		priorityRouterOld.POST("", service.Priority.Create)

		priorityRouterOld.GET("/:id", service.Priority.GetByID)
		priorityRouterOld.GET("/all", service.Priority.GetAll)

		priorityRouterOld.PUT("/:id", service.Priority.UpdateByID)

		priorityRouterOld.PATCH("/:id", service.Priority.PatchByID)

		priorityRouterOld.DELETE("/:id", service.Priority.DeleteByID)
	}

	/*
		router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
			ctx.JSON(types.Http.C300().PermanentRedirect(),
				types.EmptyResponse(
					logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
				),
			)
			ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
		})
	*/

	priorityRouter := router.Group(lastRoute)
	{
		priorityRouter.POST("", service.Priority.Create)

		priorityRouter.GET("/:id", service.Priority.GetByID)
		priorityRouter.GET("/all", service.Priority.GetAll)

		priorityRouter.PUT("/:id", service.Priority.UpdateByID)

		priorityRouter.PATCH("/:id", service.Priority.PatchByID)

		priorityRouter.DELETE("/:id", service.Priority.DeleteByID)
	}
}
