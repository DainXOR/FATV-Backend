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

		formsRouter.GET("/all", service.Forms.GetAll)
		formsRouter.GET("", service.Forms.GetAll)
		formsRouter.GET("/:id", service.Forms.GetByID)

		formQuestionsRouter := formsRouter.Group("/questions")
		{
			formQuestionsRouter.POST("", service.FormQuestions.Create)

			formQuestionsRouter.GET("/all", service.FormQuestions.GetAll)
			formQuestionsRouter.GET("", service.FormQuestions.GetAll)
			formQuestionsRouter.GET("/:id", service.FormQuestions.GetByID)

			//formQuestionsRouter.PUT("/:id", service.FormQuestions.UpdateByID)

			formQuestionsRouter.PATCH("/:id", service.FormQuestions.PatchByID)

			formQuestionsRouter.DELETE("/:id", service.FormQuestions.DeleteByID)

			formQuestionTypesRouter := formQuestionsRouter.Group("/types")
			{
				formQuestionTypesRouter.POST("", service.FormQuestionTypes.Create)

				formQuestionTypesRouter.GET("/all", service.FormQuestionTypes.GetAll)
				formQuestionTypesRouter.GET("", service.FormQuestionTypes.GetAll)
				formQuestionTypesRouter.GET("/:id", service.FormQuestionTypes.GetByID)
			}
		}

		formAnswersRouter := formsRouter.Group("/answers")
		{
			formAnswersRouter.POST("", service.FormAnswers.Create)

			formAnswersRouter.GET("/all", service.FormAnswers.GetAll)
			formAnswersRouter.GET("", service.FormAnswers.GetAll)
			formAnswersRouter.GET("/:id", service.FormAnswers.GetByID)
		}
	}

}
