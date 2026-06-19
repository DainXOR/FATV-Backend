package controller

import (
	"dainxor/atv/configs"
	"dainxor/atv/service"
	"fmt"

	"github.com/gin-gonic/gin"
)

func FormsRoutes(router *gin.Engine) {
	rv := configs.App.RoutesVersion()
	// beforeRoute := fmt.Sprintf("/api/v%d/forms", rv-1)
	lastRoute := fmt.Sprintf("/api/v%d/forms", rv)

	//formsRouterOld := router.Group(beforeRoute)
	//{ }
	formsRouter := router.Group(lastRoute)
	{
		formsRouter.POST("", service.Forms.Create)

		formsRouter.GET("/:id", service.Forms.GetByID)
		formsRouter.GET("/all", service.Forms.GetAll)

		formQuestionsRouter := formsRouter.Group("/questions")
		{
			formQuestionsRouter.POST("", service.FormQuestions.Create)

			formQuestionsRouter.GET("/:id", service.FormQuestions.GetByID)
			formQuestionsRouter.GET("/all", service.FormQuestions.GetAll)

			//formQuestionsRouter.PUT("/:id", service.FormQuestions.UpdateByID)

			formQuestionsRouter.PATCH("/:id", service.FormQuestions.PatchByID)

			formQuestionsRouter.DELETE("/:id", service.FormQuestions.DeleteByID)

			formQuestionTypesRouter := formQuestionsRouter.Group("/types")
			{
				formQuestionTypesRouter.POST("", service.FormQuestionTypes.Create)

				formQuestionTypesRouter.GET("/:id", service.FormQuestionTypes.GetByID)
				formQuestionTypesRouter.GET("/all", service.FormQuestionTypes.GetAll)
			}
		}

		formAnswersRouter := formsRouter.Group("/answers")
		{
			formAnswersRouter.POST("", service.FormAnswers.Create)

			formAnswersRouter.GET("/:id", service.FormAnswers.GetByID)
			formAnswersRouter.GET("/all", service.FormAnswers.GetAll)
		}
	}

}
