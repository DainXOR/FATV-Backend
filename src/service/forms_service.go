package service

import (
	"dainxor/atv/dao"
	"dainxor/atv/logger"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"

	"github.com/gin-gonic/gin"
)

type formsNS struct{}

var Forms formsNS

func (formsNS) Create(c *gin.Context) {
	var body models.FormCreate

	if err := c.ShouldBindJSON(&body); err != nil {
		expected := utils.StructToString(body)
		logger.Error(err.Error())
		logger.Error("Failed to create form: JSON request body is invalid")
		logger.Error("Expected body: ", expected)

		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid request body",
				"Expected body: "+expected,
			),
		)
		return
	}

	logger.Debug("Creating form in db: ", utils.StructToTagString(body, "json"))
	logger.Debugf("Fields: %+v", body)

	result := dao.Forms.Create(body)

	if result.IsErr() {
		logger.Error("Failed to create form in MongoDB: ", result.Error())
		handleErrorAnswer(c, result.Error())
		return
	}

	object := result.Value()
	c.JSON(types.Http.C200().Created(),
		types.Response(
			object.ToResponse(),
			"",
		),
	)
}

func (formsNS) GetByID(c *gin.Context) {
	id := c.Param("id")
	logger.Debug("Getting form by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())

	result := dao.Forms.GetByID(id, filter)

	if result.IsErr() {
		handleErrorAnswer(c, result.Error())
		return
	}

	form := result.Value()
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			form.ToResponse(),
			"",
		),
	)
}
func (formsNS) GetAll(c *gin.Context) {
	filter := models.Filter.Create(c.Request.URL.Query())
	result := dao.Forms.GetAll(filter)

	if result.IsErr() {
		handleErrorAnswer(c, result.Error())
		return
	}

	objects := utils.Map(result.Value(), models.FormDB.ToResponse)
	if len(objects) == 0 {
		logger.Warning("No form found")
		c.JSON(types.Http.C400().NotFound(),
			types.EmptyResponse(
				"No form found",
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

func (formsNS) UpdateByID(c *gin.Context) {}

func (formsNS) PatchByID(c *gin.Context) {}

func (formsNS) DeleteByID(c *gin.Context) {}
