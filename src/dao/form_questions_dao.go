package dao

import (
	"dainxor/atv/configs"
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"
)

type formQuestionsNS struct{}

var FormQuestions formQuestionsNS

func (formQuestionsNS) Create(t models.FormQuestionCreate) types.Result[models.FormQuestionDB] {
	questionDB, err := t.ToInsert()
	if err != nil {
		logger.Warning("Error converting form question to DB model:", err)
		return types.ResultErr[models.FormQuestionDB](err)
	}

	resultCreate := configs.DB.InsertOne(questionDB)

	if resultCreate.IsErr() {
		logger.Warning("Failed to create form question in DB:", resultCreate.Error())
		return types.ResultErr[models.FormQuestionDB](resultCreate.Error())
	}

	questionDB.ID = resultCreate.Value()
	return types.ResultOk(*questionDB)
}

func (formQuestionsNS) GetByID(id string, filter models.FilterObject) types.Result[models.FormQuestionDB] {
	oid, err := models.ID.ToDB(id)

	if err != nil {
		logger.Warning("Failed to convert ID to ObjectID: ", err)
		httpErr := types.Error(
			types.Http.C400().UnprocessableEntity(),
			"Invalid value",
			"Invalid ID format: "+err.Error(),
			"Form Question ID: "+id,
		)
		return types.ResultErr[models.FormQuestionDB](&httpErr)
	}

	filter = models.Filter.AddPart(filter, models.Filter.ID(oid))
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())
	var questionType models.FormQuestionDB

	resultGet := configs.DB.FindOne(filter, questionType)
	if resultGet.IsErr() {
		logger.Warning("Failed to get form question by ID: ", resultGet.Error())

		return types.ResultErr[models.FormQuestionDB](resultGet.Error())
	}
	questionType = resultGet.Value().(models.FormQuestionDB)
	return types.ResultOk(questionType)
}
func (formQuestionsNS) GetAll(filter models.FilterObject) types.Result[[]models.FormQuestionDB] {
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())

	resultObjects := configs.DB.FindAll(filter, models.FormQuestionDB{})
	if resultObjects.IsErr() {
		logger.Warning("Failed to get all objects: ", resultObjects.Error())
		return types.ResultErr[[]models.FormQuestionDB](resultObjects.Error())
	}

	objectsDB := utils.Map(resultObjects.Value(), models.InterfaceTo[models.FormQuestionDB])
	logger.Debug("Retrieved", len(objectsDB), "objects from db")

	return types.ResultOk(objectsDB)
}

func (formQuestionsNS) PatchByID(id string, model models.FormQuestionCreate, filter models.FilterObject) types.Result[models.FormQuestionDB] {
	oid, err := models.ID.ToDB(id)

	if err != nil {
		logger.Warning("Failed to convert ID to ObjectID: ", err)
		httpErr := types.Error(
			types.Http.C400().UnprocessableEntity(),
			"Invalid value",
			"Invalid ID format: "+err.Error(),
			"Form Question ID: "+id,
		)
		return types.ResultErr[models.FormQuestionDB](&httpErr)
	}

	filter = models.Filter.AddPart(filter, models.Filter.ID(oid))
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())
	question, err := model.ToInsert()
	if err != nil {
		logger.Warning("Failed to patch question:", err)
		httpErr := types.Error(
			types.Http.C500().InternalServerError(),
			"Internal error",
			err.Error(),
			"Form Question ID: "+id,
		)
		return types.ResultErr[models.FormQuestionDB](&httpErr)
	}

	resultGet := configs.DB.PatchOne(filter, question)
	if resultGet != nil {
		logger.Warning("Failed to get form question type by ID: ", resultGet)

		return types.ResultErr[models.FormQuestionDB](resultGet)
	}

	return types.ResultOk(*question)
}

func (formQuestionsNS) SoftDeleteByID(id string, filter models.FilterObject) error {
	oid, err := models.ID.ToDB(id)

	if err != nil {
		logger.Warning("Failed to convert ID to ObjectID: ", err)
		httpErr := types.Error(
			types.Http.C400().UnprocessableEntity(),
			"Invalid value",
			"Invalid ID format: "+err.Error(),
			"Form Question ID: "+id,
		)
		return &httpErr
	}

	filter = models.Filter.AddPart(filter, models.Filter.ID(oid))
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())
	err = configs.DB.SoftDeleteOne(filter, models.FormQuestionDB{})

	if err != nil {
		logger.Warning("Failed to soft delete question:", err)
		httpErr := types.Error(
			types.Http.C500().InternalServerError(),
			"Internal error",
			err.Error(),
			"Form Question ID: "+id,
		)
		return &httpErr
	}

	return nil
}
