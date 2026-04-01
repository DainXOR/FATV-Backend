package service

import (
	"dainxor/atv/dao"
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"

	"github.com/gin-gonic/gin"
)

type formAnswersNS struct{}

var FormAnswers formAnswersNS

func (formAnswersNS) Create(c *gin.Context) {
	var body models.FormAnswerCreate

	if err := c.ShouldBindJSON(&body); err != nil {
		expected := utils.StructToString(body)
		logger.Error(err.Error())
		logger.Error("Failed to create form answer: JSON request body is invalid")
		logger.Error("Expected body: ", expected)

		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid request body",
				"Expected body: "+expected,
			),
		)
		return
	}

	logger.Debug("Creating form answer in db: ", body)

	result := dao.FormAnswers.Create(body)

	if result.IsErr() {
		logger.Error("Failed to create form answer in db: ", result.Error())
		handleErrorAnswer(c, result.Error())
		return
	}

	answers := result.Value()
	c.JSON(
		types.Http.C200().Created(),
		types.Response(
			answers.ToResponse(),
			"",
		),
	)

	go generateAlert(answers)
}

func (formAnswersNS) GetByID(c *gin.Context) {
	id := c.Param("id")
	logger.Debug("Getting answer by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())

	result := dao.FormAnswers.GetByID(id, filter)

	if result.IsErr() {
		handleErrorAnswer(c, result.Error())
		return
	}

	answer := result.Value()
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			answer.ToResponse(),
			"",
		),
	)
}
func (formAnswersNS) GetAll(c *gin.Context) {
	filter := models.Filter.Create(c.Request.URL.Query())
	result := dao.FormAnswers.GetAll(filter)

	if result.IsErr() {
		handleErrorAnswer(c, result.Error())
		return
	}

	objects := utils.Map(result.Value(), models.FormAnswerDB.ToResponse)
	if len(objects) == 0 {
		logger.Warning("No answer found")
		c.JSON(types.Http.C400().NotFound(),
			types.EmptyResponse(
				"No answer found",
			))
		return
	}
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			objects,
			"",
		),
	)
}

func (formAnswersNS) UpdateByID(c *gin.Context) {}

func (formAnswersNS) PatchByID(c *gin.Context) {}

func (formAnswersNS) DeleteByID(c *gin.Context) {}

func generateAlert(answers models.FormAnswerDB) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic in background worker: ", r)
		}
	}()

	formResult := dao.Forms.GetByID(answers.ID.Hex(), models.Filter.Empty())
	if formResult.IsErr() {
		logger.Error("Failed to get form in db: ", formResult.Error())
		// Send error data
		return
	}

	form := formResult.Value()
	riskValue := 0

	for _, questionInfo := range form.QuestionsInfo {
		questionResult := dao.FormQuestions.GetByID(questionInfo.IDQuestion.Hex(), models.Filter.Empty())

		if questionResult.IsErr() {
			logger.Error("Failed to get question in db: ", questionResult.Error())
			logger.Error("Question ID:", questionInfo.IDQuestion.Hex())
			continue
		}

		question := questionResult.Value()
		questionWeight := questionInfo.Weight
		answersWeights := question.Options

		riskValue = utils.Reduce(answersWeights, func(acc int, o models.Option) int {
			utils.Map(answers.Answers, func(a models.Answers[models.DBID]) string {
				return a.ProvidedAnswers
			})
			//utils.Contains(, o.Text)
			return 0
		}, riskValue)

	}

	//configs.WebHooks.SendTo("", "")
}
