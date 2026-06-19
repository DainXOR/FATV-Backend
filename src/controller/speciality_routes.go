package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func SpecialitiesRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/speciality", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/specialities", rv)

	specialityRouterOld := router.Group(beforeRoute)
	{
		specialityRouterOld.POST("", service.Speciality.Create)

		specialityRouterOld.GET("/:id", service.Speciality.GetByID)
		specialityRouterOld.GET("/all", service.Speciality.GetAll)

		specialityRouterOld.PUT("/:id", service.Speciality.UpdateByID)

		specialityRouterOld.PATCH("/:id", service.Speciality.PatchByID)

		specialityRouterOld.DELETE("/:id", service.Speciality.DeleteByID)
	}

	//router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
	//	ctx.JSON(types.Http.C300().PermanentRedirect(),
	//		types.EmptyResponse(
	//			logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
	//		),
	//	)
	//	ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
	//})

	specialityRouter := router.Group(lastRoute)
	{
		specialityRouter.POST("/", service.Speciality.Create)

		specialityRouter.GET("/:id", service.Speciality.GetByID)
		specialityRouter.GET("/all", service.Speciality.GetAll)

		specialityRouter.PUT("/:id", service.Speciality.UpdateByID)

		specialityRouter.PATCH("/:id", service.Speciality.PatchByID)

		specialityRouter.DELETE("/:id", service.Speciality.DeleteByID)
	}
}
