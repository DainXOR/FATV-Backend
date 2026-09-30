package controller

import (
	"errors"
	"net/http"
	"time"

	"dainxor/atv/formaccess"
	"dainxor/atv/middleware"
	"dainxor/atv/models"

	"github.com/gin-gonic/gin"
)

type issueInvitationsBody struct {
	FormID     string    `json:"form_id" binding:"required"`
	StudentIDs []string  `json:"student_ids" binding:"required,min=1"`
	ExpiresAt  time.Time `json:"expires_at" binding:"required"`
}
type submitFormBody struct {
	Answers models.Answers[string] `json:"answers" binding:"required"`
}

func FormAccessRoutes(router *gin.Engine) {
	base := "/api/v" + routeVersionString()
	invitations := router.Group(base + "/form-invitations")
	{
		invitations.POST("", issueInvitations)
		invitations.GET("", listInvitations)
		invitations.DELETE("/:id", revokeInvitation)
	}
	public := router.Group(base + "/public/forms")
	{
		public.GET("/:token", getPublicForm)
		public.GET("/:token/questions/:questionId", getPublicQuestion)
		public.GET("/:token/question-types/:typeId", getPublicQuestionType)
		public.POST("/:token/answers", submitPublicForm)
	}
}

func issueInvitations(c *gin.Context) {
	var body issueInvitationsBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "form_id, student_ids, and expires_at are required"})
		return
	}
	actor, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	result, err := formaccess.Default.Issue(body.FormID, body.StudentIDs, body.ExpiresAt, actor.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func listInvitations(c *gin.Context) {
	items, err := formaccess.Default.List(c.Query("form_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form id or unable to list invitations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"invitations": items})
}

func revokeInvitation(c *gin.Context) {
	actor, _ := middleware.CurrentUser(c)
	if err := formaccess.Default.Revoke(c.Param("id"), actor.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invitation not found or unavailable"})
		return
	}
	c.Status(http.StatusNoContent)
}

func getPublicForm(c *gin.Context) {
	form, err := formaccess.Default.GetForm(c.Param("token"))
	if err != nil {
		publicUnavailable(c)
		return
	}
	setPublicFormHeaders(c)
	c.JSON(http.StatusOK, gin.H{"data": form})
}

func getPublicQuestion(c *gin.Context) {
	question, err := formaccess.Default.GetQuestion(c.Param("token"), c.Param("questionId"))
	if err != nil {
		publicUnavailable(c)
		return
	}
	setPublicFormHeaders(c)
	c.JSON(http.StatusOK, gin.H{"data": question})
}

func getPublicQuestionType(c *gin.Context) {
	questionType, err := formaccess.Default.GetQuestionType(c.Param("token"), c.Param("typeId"))
	if err != nil {
		publicUnavailable(c)
		return
	}
	setPublicFormHeaders(c)
	c.JSON(http.StatusOK, gin.H{"data": questionType})
}

func submitPublicForm(c *gin.Context) {
	var body submitFormBody
	if c.ShouldBindJSON(&body) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form answers"})
		return
	}
	if err := formaccess.Default.Submit(c.Param("token"), body.Answers); err != nil {
		if errors.Is(err, formaccess.ErrUnavailable) {
			publicUnavailable(c)
			return
		}
		if errors.Is(err, formaccess.ErrSubmissionFailed) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "submission could not be saved"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "answers are invalid or required questions are missing"})
		return
	}
	setPublicFormHeaders(c)
	c.JSON(http.StatusCreated, gin.H{"message": "Form submitted successfully"})
}

func setPublicFormHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
}

func publicUnavailable(c *gin.Context) {
	setPublicFormHeaders(c)
	c.JSON(http.StatusNotFound, gin.H{"error": "This form is unavailable."})
}
