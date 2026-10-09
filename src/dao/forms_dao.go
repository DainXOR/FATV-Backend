package dao

import (
	"dainxor/atv/configs"
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"
)

type formsNS struct{}

var Forms formsNS

func (formsNS) Create(t models.FormCreate) types.Result[models.FormDB] {
	resultFormDB := t.ToInsert()
	if resultFormDB.IsErr() {
		logger.Warning("Error converting form to DB model:", resultFormDB.Error())
		return types.ResultErr[models.FormDB](resultFormDB.Error())
	}

	formDB := resultFormDB.Value()
	resultCreate := configs.DB.InsertOne(formDB)

	if resultCreate.IsErr() {
		logger.Warning("Failed to create form in MongoDB:", resultCreate.Error())
		return types.ResultErr[models.FormDB](resultCreate.Error())
	}

	formDB.ID = resultCreate.Value()
	return types.ResultOk(formDB)
}

func (formsNS) GetByID(id string, filter models.FilterObject) types.Result[models.FormDB] {
	oid, err := models.ID.ToBson(id)

	if err != nil {
		logger.Warning("Failed to convert ID to ObjectID: ", err)
		httpErr := types.Error(
			types.Http.C400().UnprocessableEntity(),
			"Invalid value",
			"Invalid ID format: "+err.Error(),
			"Form ID: "+id,
		)
		return types.ResultErr[models.FormDB](&httpErr)
	}

	filter = models.Filter.AddPart(filter, models.Filter.ID(oid))
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())
	var formDB models.FormDB

	resultGet := configs.DB.FindOne(filter, formDB)
	if resultGet.IsErr() {
		logger.Warning("Failed to get form by ID: ", resultGet.Error())

		return types.ResultErr[models.FormDB](resultGet.Error())
	}
	formDB = resultGet.Value().(models.FormDB)
	return types.ResultOk(formDB)
}
func (formsNS) GetAll(filter models.FilterObject) types.Result[[]models.FormDB] {
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())

	resultObjects := configs.DB.FindAll(filter, models.FormDB{})
	if resultObjects.IsErr() {
		logger.Warning("Failed to get all objects: ", resultObjects.Error())
		return types.ResultErr[[]models.FormDB](resultObjects.Error())
	}

	objectsDB := utils.Map(resultObjects.Value(), models.InterfaceTo[models.FormDB])
	logger.Debug("Retrieved", len(objectsDB), "objects from db")

	return types.ResultOk(objectsDB)
}

func (formsNS) PatchByID(id string, form models.FormCreate, filter models.FilterObject) types.Result[models.FormDB] {
	oid, err := models.ID.ToDB(id)
	if err != nil {
		httpErr := types.Error(types.Http.C400().UnprocessableEntity(), "Invalid form ID", err.Error())
		return types.ResultErr[models.FormDB](&httpErr)
	}
	update := form.ToUpdate()
	if update.IsErr() {
		return types.ResultErr[models.FormDB](update.Error())
	}
	filter = models.Filter.AddPart(filter, models.Filter.ID(oid))
	filter = models.Filter.AddPart(filter, models.Filter.NotDeleted())
	if err := configs.DB.PatchOne(filter, update.Value()); err != nil {
		return types.ResultErr[models.FormDB](err)
	}
	return (formsNS{}).GetByID(id, models.Filter.Empty())
}
