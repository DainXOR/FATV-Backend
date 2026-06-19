package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func StudentsRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	beforeRoute := fmt.Sprintf("/api/v%d/student", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/students", rv)

	studentRouterOld := router.Group(beforeRoute)
	{
		studentRouterOld.GET("/:id", service.Student.GetByID)
		studentRouterOld.GET("/all", service.Student.GetAll)

		studentRouterOld.POST("", service.Student.Create)

		studentRouterOld.PUT("/:id", service.Student.UpdateByID)

		studentRouterOld.PATCH("/:id", service.Student.PatchByID)

		studentRouterOld.DELETE("/:id", service.Student.DeleteByID)
		//studentRouter.DELETE("/permanent-delete/:id/:confirm", controller.Student.ForceDeleteByID)

	}

	//router.Group(beforeRoute).Any("", func(ctx *gin.Context) {
	//	ctx.JSON(types.Http.C300().PermanentRedirect(),
	//		types.EmptyResponse(
	//			logger.DeprecateMsg(types.V("0.2.0"), types.V("0.3.0"), "Use", lastRoute, "instead"),
	//		),
	//	)
	//	ctx.Redirect(types.Http.C300().PermanentRedirect(), lastRoute)
	//})

	studentRouter := router.Group(lastRoute)
	{
		studentRouter.GET("/:id", service.Student.GetByID)
		studentRouter.GET("/all", service.Student.GetAll)

		studentRouter.POST("", service.Student.Create)

		studentRouter.PUT("/:id", service.Student.UpdateByID)

		studentRouter.PATCH("/:id", service.Student.PatchByID)

		studentRouter.DELETE("/:id", service.Student.DeleteByID)
		//studentRouter.DELETE("/permanent-delete/:id/:confirm", controller.Student.ForceDeleteByID)

	}
}
