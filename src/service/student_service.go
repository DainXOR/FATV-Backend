package service

import (
	"dainxor/atv/dao"
	"dainxor/atv/logger"
	"dainxor/atv/middleware"
	"dainxor/atv/models"
	"dainxor/atv/types"
	"dainxor/atv/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type studentType struct{}

var Student studentType

func (studentType) Create(c *gin.Context) {
	var body models.StudentCreate

	if err := c.ShouldBindJSON(&body); err != nil {
		expected := utils.StructToString(body)
		logger.Error(err.Error())
		logger.Error("Failed to create student: JSON request body is invalid")
		logger.Error("Expected body: ", expected)

		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid request body",
				"Expected body: "+expected,
			),
		)
		return
	}

	processID, ok := middleware.ActiveProcessID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "active process required"})
		return
	}
	body.ProcessID = processID.Hex()
	logger.Debug("Creating student in MongoDB: ", body)

	result := dao.Student.Create(body)

	if result.IsErr() {
		logger.Error("Failed to create student in MongoDB: ", result.Error())
		err := result.Error()
		httpErr := err.(*types.HttpError)
		c.JSON(httpErr.Code,
			types.EmptyResponse(
				httpErr.Msg(),
				httpErr.Details(),
			),
		)
		return
	}

	student := result.Value()
	c.JSON(types.Http.C200().Created(),
		types.Response(
			student.ToResponse(),
			"",
		),
	)
}

func (studentType) GetByID(c *gin.Context) {
	id := c.Param("id")
	logger.Debug("Getting student by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())
	if processID, ok := middleware.ActiveProcessID(c); ok {
		filter = models.Filter.AddPart(filter, models.Filter.Of("process_id", processID))
	}

	result := dao.Student.GetByID(id, filter)

	if result.IsErr() {
		err := result.Error()
		cerror := err.(*types.HttpError)
		c.JSON(cerror.Code,
			types.EmptyResponse(
				cerror.Msg(),
				cerror.Details(),
			),
		)
		return
	}

	student := result.Value()
	c.JSON(http.StatusOK,
		types.Response(
			student.ToResponse(),
			"",
		),
	)
}
func (studentType) GetAll(c *gin.Context) {
	filter := models.Filter.Create(c.Request.URL.Query())
	if processID, ok := middleware.ActiveProcessID(c); ok {
		filter = models.Filter.AddPart(filter, models.Filter.Of("process_id", processID))
	}
	result := dao.Student.GetAll(filter)

	if result.IsErr() {
		err := result.Error().(*types.HttpError)
		c.JSON(err.Code,
			types.EmptyResponse(
				err.Msg(),
				err.Details(),
			),
		)
		return
	}

	students := utils.Map(result.Value(), models.StudentDB.ToResponse)
	if len(students) == 0 {
		logger.Warning("No students found in MongoDB database")
		c.JSON(types.Http.C400().NotFound(),
			types.EmptyResponse(
				"No students found",
			))
		return
	}
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			students,
			"",
		),
	)
}

func (studentType) UpdateByID(c *gin.Context) {
	var body models.StudentCreate

	if err := c.ShouldBindJSON(&body); err != nil {
		expected := utils.StructToString(body)
		logger.Error(err.Error())
		logger.Error("Failed to update student: JSON request body is invalid")
		logger.Error("Expected body: ", expected)

		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid request body",
				"Expected body: "+expected,
			),
		)
		return
	}

	id := c.Param("id")
	logger.Debug("Updating student by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())
	if processID, ok := middleware.ActiveProcessID(c); ok {
		filter = models.Filter.AddPart(filter, models.Filter.Of("process_id", processID))
	}

	result := dao.Student.UpdateByID(id, body, filter)
	if result.IsErr() {
		err := result.Error()
		cerror := err.(*types.HttpError)
		c.JSON(cerror.Code, err)
		return
	}

	student := result.Value()
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			student.ToResponse(),
			"",
		),
	)
}

func (studentType) PatchByID(c *gin.Context) {
	var body models.StudentCreate

	if err := c.ShouldBindJSON(&body); err != nil {
		expected := utils.StructToString(body)
		logger.Error(err.Error())
		logger.Error("Failed to patch student: JSON request body is invalid")
		logger.Error("Expected body: ", expected)

		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid request body",
				"Expected body: "+expected,
			),
		)
		return
	}

	id := c.Param("id")
	filter := models.Filter.Create(c.Request.URL.Query())
	if processID, ok := middleware.ActiveProcessID(c); ok {
		filter = models.Filter.AddPart(filter, models.Filter.Of("process_id", processID))
	}

	result := dao.Student.PatchByID(id, body, filter)

	if result.IsErr() {
		err := result.Error()
		cerror := err.(*types.HttpError)
		c.JSON(cerror.Code,
			types.EmptyResponse(
				cerror.Msg(),
				cerror.Details(),
			),
		)
		return
	}

	student := result.Value()
	c.JSON(types.Http.C200().Ok(),
		types.Response(
			student.ToResponse(),
			"",
		),
	)
}

func (studentType) DeleteByID(c *gin.Context) {
	id := c.Param("id")
	logger.Debug("Deleting student by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())
	if processID, ok := middleware.ActiveProcessID(c); ok {
		filter = models.Filter.AddPart(filter, models.Filter.Of("process_id", processID))
	}

	result := dao.Student.DeleteByID(id, filter)

	if result.IsErr() {
		err := result.Error()
		cerror := err.(*types.HttpError)
		c.JSON(cerror.Code,
			types.EmptyResponse(
				cerror.Msg(),
				cerror.Details(),
			),
		)
		return
	}

	data := result.Value().ToResponse()

	c.JSON(types.Http.C200().Accepted(),
		types.Response(
			data,
			"Student marked for deletion",
		),
	)
}
func (studentType) ForceDeleteByID(c *gin.Context) {
	confirm := c.Param("confirm")
	if confirm != "delete-permanently" {
		c.JSON(types.Http.C400().BadRequest(),
			types.EmptyResponse(
				"Invalid confirmation parameter",
				"Use 'delete-permanently' to confirm deletion",
			),
		)
		return
	}

	id := c.Param("id")
	logger.Info("Force deleting student by ID: ", id)
	filter := models.Filter.Create(c.Request.URL.Query())

	result := dao.Student.DeletePermanentByID(id, filter)

	if result.IsErr() {
		err := result.Error()
		cerror := err.(*types.HttpError)
		c.JSON(cerror.Code,
			types.EmptyResponse(
				cerror.Msg(),
				cerror.Details(),
			),
		)
		return
	}

	data := result.Value().ToResponse()

	c.JSON(types.Http.C200().Ok(),
		types.Response(
			data,
			"Student deleted permanently",
		),
	)
}
