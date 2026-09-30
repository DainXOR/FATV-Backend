package formaccess

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"dainxor/atv/auth"
	"dainxor/atv/configs"
	"dainxor/atv/dao"
	"dainxor/atv/emailer"
	"dainxor/atv/logger"
	"dainxor/atv/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	ErrUnavailable      = errors.New("form invitation unavailable")
	ErrSubmissionFailed = errors.New("form submission failed")
)

type InvitationStatus string

const (
	InvitationPending   InvitationStatus = "pending"
	InvitationSubmitted InvitationStatus = "submitted"
	InvitationRevoked   InvitationStatus = "revoked"
	InvitationExpired   InvitationStatus = "expired"
)

type InvitationSummary struct {
	ID          string           `json:"id"`
	FormID      string           `json:"form_id"`
	StudentID   string           `json:"student_id"`
	ExpiresAt   time.Time        `json:"expires_at"`
	CreatedAt   time.Time        `json:"created_at"`
	Status      InvitationStatus `json:"status"`
	SubmittedAt time.Time        `json:"submitted_at,omitempty"`
}

type IssueResult struct {
	Sent   int `json:"sent"`
	Failed int `json:"failed"`
}

type Service struct{}

var Default Service

func (Service) Issue(formID string, studentIDs []string, expiresAt time.Time, actor models.DBID) (IssueResult, error) {
	var summary IssueResult
	formOID, err := models.ID.ToDB(formID)
	if err != nil || len(studentIDs) == 0 || len(studentIDs) > 500 {
		return summary, fmt.Errorf("invalid form or student list (maximum 500)")
	}
	now := time.Now().UTC()
	if !expiresAt.After(now.Add(5*time.Minute)) || expiresAt.After(now.Add(30*24*time.Hour)) {
		return summary, fmt.Errorf("invitation expiry must be between 5 minutes and 30 days")
	}
	baseURL := strings.TrimRight(os.Getenv("FRONTEND_BASE_URL"), "/")
	if baseURL == "" {
		return summary, fmt.Errorf("FRONTEND_BASE_URL is not configured")
	}
	mailer, err := emailer.NewFromEnv()
	if err != nil {
		return summary, err
	}
	formResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: formOID}}, models.FormDB{})
	if formResult.IsErr() {
		return summary, fmt.Errorf("form not found")
	}
	form := models.InterfaceTo[models.FormDB](formResult.Value())
	if !form.DeletedAt.IsZero() {
		return summary, fmt.Errorf("form is unavailable")
	}

	seen := map[string]bool{}
	for _, studentID := range studentIDs {
		studentOID, parseErr := models.ID.ToDB(studentID)
		if parseErr != nil || seen[studentID] {
			summary.Failed++
			continue
		}
		seen[studentID] = true
		studentResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: studentOID}}, models.StudentDB{})
		if studentResult.IsErr() {
			summary.Failed++
			continue
		}
		student := models.InterfaceTo[models.StudentDB](studentResult.Value())
		if !student.DeletedAt.IsZero() {
			summary.Failed++
			continue
		}
		address, parseErr := emailer.ParseAddress(strings.TrimSpace(student.InstitutionEmail))
		if parseErr != nil {
			address, parseErr = emailer.ParseAddress(strings.TrimSpace(student.PersonalEmail))
		}
		if parseErr != nil {
			summary.Failed++
			continue
		}

		token, tokenErr := newToken()
		if tokenErr != nil {
			return summary, tokenErr
		}
		invitation := models.FormInvitationDB{
			FormID: formOID, StudentID: studentOID, TokenHash: tokenHash(token),
			ExpiresAt: expiresAt.UTC(), CreatedBy: actor, CreatedAt: now,
			SubmissionState: models.InvitationStateOpen,
		}
		insert := configs.DB.InsertOne(&invitation)
		if insert.IsErr() {
			summary.Failed++
			logger.Warning("Could not create form invitation:", insert.Error())
			continue
		}
		invitation.ID = insert.Value()
		link := baseURL + "/student-form/" + url.PathEscape(token)
		if sendErr := mailer.SendFormInvitation(address, link, expiresAt.UTC().Format(time.RFC3339)); sendErr != nil {
			_ = configs.DB.PatchOne(bson.D{{Key: "_id", Value: invitation.ID}},
				models.FormInvitationPatch{RevokedAt: time.Now().UTC()})
			auth.Default.RecordAudit(actor, "form_invitation_created", invitation.ID.Hex(), "delivery_failed")
			summary.Failed++
			logger.Warning("Form invitation email delivery failed:", sendErr)
			continue
		}
		auth.Default.RecordAudit(actor, "form_invitation_created", invitation.ID.Hex(), "sent")
		summary.Sent++
		logger.Info("Form invitation sent", "form_id", formOID.Hex(), "student_id", studentOID.Hex(), "invitation_id", invitation.ID.Hex())
	}
	return summary, nil
}

func (Service) List(formID string) ([]InvitationSummary, error) {
	formOID, err := models.ID.ToDB(formID)
	if err != nil {
		return nil, fmt.Errorf("invalid form id")
	}
	result := configs.DB.FindAll(bson.D{{Key: "form_id", Value: formOID}}, models.FormInvitationDB{})
	if result.IsErr() {
		return nil, result.Error()
	}
	now := time.Now().UTC()
	items := make([]InvitationSummary, 0, len(result.Value()))
	for _, item := range result.Value() {
		inv := models.InterfaceTo[models.FormInvitationDB](item)
		status := InvitationPending
		switch {
		case inv.SubmissionState == models.InvitationStateSubmitted || !inv.SubmittedAt.IsZero():
			status = InvitationSubmitted
		case !inv.RevokedAt.IsZero():
			status = InvitationRevoked
		case !inv.ExpiresAt.After(now):
			status = InvitationExpired
		}
		items = append(items, InvitationSummary{ID: inv.ID.Hex(), FormID: inv.FormID.Hex(),
			StudentID: inv.StudentID.Hex(), ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
			Status: status, SubmittedAt: inv.SubmittedAt})
	}
	return items, nil
}

func (Service) Revoke(id string, actor models.DBID) error {
	oid, err := models.ID.ToDB(id)
	if err != nil {
		return fmt.Errorf("invalid invitation id")
	}
	now := time.Now().UTC()
	err = configs.DB.PatchOne(bson.D{{Key: "_id", Value: oid}, {Key: "revoked_at", Value: nil},
		{Key: "submission_state", Value: bson.D{{Key: "$in", Value: bson.A{models.InvitationStateOpen, nil}}}}},
		models.FormInvitationPatch{RevokedAt: now})
	if err == nil {
		auth.Default.RecordAudit(actor, "form_invitation_revoked", oid.Hex(), "success")
	}
	return err
}

func (Service) GetForm(token string) (models.FormResponse, error) {
	inv, err := findValidInvitation(token)
	if err != nil {
		return models.FormResponse{}, err
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: inv.FormID}}, models.FormDB{})
	if result.IsErr() {
		return models.FormResponse{}, ErrUnavailable
	}
	form := models.InterfaceTo[models.FormDB](result.Value())
	if !form.DeletedAt.IsZero() {
		return models.FormResponse{}, ErrUnavailable
	}
	return form.ToResponse(), nil
}

func (Service) GetQuestion(token, questionID string) (models.FormQuestionResponse, error) {
	inv, form, err := findForm(token)
	if err != nil {
		return models.FormQuestionResponse{}, err
	}
	questionOID, err := models.ID.ToDB(questionID)
	if err != nil || !containsQuestion(form, questionOID) {
		return models.FormQuestionResponse{}, ErrUnavailable
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: questionOID}}, models.FormQuestionDB{})
	if result.IsErr() {
		return models.FormQuestionResponse{}, ErrUnavailable
	}
	question := models.InterfaceTo[models.FormQuestionDB](result.Value())
	if !question.DeletedAt.IsZero() {
		return models.FormQuestionResponse{}, ErrUnavailable
	}
	_ = inv
	return question.ToResponse(), nil
}

func (Service) GetQuestionType(token, typeID string) (models.FormQuestionTypeResponse, error) {
	_, form, err := findForm(token)
	if err != nil {
		return models.FormQuestionTypeResponse{}, err
	}
	typeOID, err := models.ID.ToDB(typeID)
	if err != nil {
		return models.FormQuestionTypeResponse{}, ErrUnavailable
	}
	used := false
	for _, questionInfo := range form.QuestionsInfo {
		qr := configs.DB.FindOne(bson.D{{Key: "_id", Value: questionInfo.IDQuestion}}, models.FormQuestionDB{})
		if qr.IsOk() && models.InterfaceTo[models.FormQuestionDB](qr.Value()).IDQuestionType == typeOID {
			used = true
			break
		}
	}
	if !used {
		for _, questions := range form.Sections {
			for _, questionInfo := range questions {
				qr := configs.DB.FindOne(bson.D{{Key: "_id", Value: questionInfo.IDQuestion}}, models.FormQuestionDB{})
				if qr.IsOk() && models.InterfaceTo[models.FormQuestionDB](qr.Value()).IDQuestionType == typeOID {
					used = true
					break
				}
			}
			if used {
				break
			}
		}
	}
	if !used {
		return models.FormQuestionTypeResponse{}, ErrUnavailable
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: typeOID}}, models.FormQuestionTypeDB{})
	if result.IsErr() {
		return models.FormQuestionTypeResponse{}, ErrUnavailable
	}
	return models.InterfaceTo[models.FormQuestionTypeDB](result.Value()).ToResponse(), nil
}

func (Service) Submit(token string, answers models.Answers[string]) error {
	inv, form, err := findForm(token)
	if err != nil {
		return err
	}
	if err := validateAnswers(form, answers); err != nil {
		return err
	}
	now := time.Now().UTC()
	claimFilter := bson.D{
		{Key: "_id", Value: inv.ID},
		{Key: "revoked_at", Value: nil},
		{Key: "submission_state", Value: bson.D{{Key: "$in", Value: bson.A{models.InvitationStateOpen, nil}}}},
		{Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now}}},
	}
	if err := configs.DB.PatchOne(claimFilter, models.FormInvitationPatch{
		SubmittedAt: now, SubmissionState: models.InvitationStateSubmitted,
	}); err != nil {
		if errors.Is(err, configs.DBErr.NotFound()) || errors.Is(err, configs.DBErr.NotModified()) {
			return ErrUnavailable
		}
		return fmt.Errorf("%w: %v", ErrSubmissionFailed, err)
	}
	created := dao.FormAnswers.Create(models.FormAnswerCreate{
		IDForm: inv.FormID.Hex(), Answers: answers, InvitationID: inv.ID,
	})
	if created.IsErr() {
		if rollbackErr := configs.DB.PatchOne(bson.D{{Key: "_id", Value: inv.ID},
			{Key: "submission_state", Value: models.InvitationStateSubmitted}, {Key: "submitted_at", Value: now}},
			models.FormInvitationRollbackPatch{SubmittedAt: time.Time{}, SubmissionState: models.InvitationStateOpen}); rollbackErr != nil {
			logger.Error("Could not release form invitation after failed submission:", rollbackErr)
		}
		return fmt.Errorf("%w: %v", ErrSubmissionFailed, created.Error())
	}
	auth.Default.RecordAudit(models.DBID{}, "student_form_submitted", inv.ID.Hex(), "success")
	logger.Info("Student form submitted", "form_id", inv.FormID.Hex(), "invitation_id", inv.ID.Hex())
	return nil
}

func findForm(token string) (models.FormInvitationDB, models.FormDB, error) {
	inv, err := findValidInvitation(token)
	if err != nil {
		return models.FormInvitationDB{}, models.FormDB{}, err
	}
	result := configs.DB.FindOne(bson.D{{Key: "_id", Value: inv.FormID}}, models.FormDB{})
	if result.IsErr() {
		return models.FormInvitationDB{}, models.FormDB{}, ErrUnavailable
	}
	form := models.InterfaceTo[models.FormDB](result.Value())
	if !form.DeletedAt.IsZero() {
		return models.FormInvitationDB{}, models.FormDB{}, ErrUnavailable
	}
	return inv, form, nil
}

func findValidInvitation(token string) (models.FormInvitationDB, error) {
	if token == "" || len(token) > 256 {
		return models.FormInvitationDB{}, ErrUnavailable
	}
	result := configs.DB.FindOne(bson.D{{Key: "token_hash", Value: tokenHash(token)}}, models.FormInvitationDB{})
	if result.IsErr() {
		return models.FormInvitationDB{}, ErrUnavailable
	}
	inv := models.InterfaceTo[models.FormInvitationDB](result.Value())
	if invitationUnavailable(inv, time.Now().UTC()) {
		return models.FormInvitationDB{}, ErrUnavailable
	}
	return inv, nil
}

func invitationUnavailable(inv models.FormInvitationDB, now time.Time) bool {
	return !inv.ExpiresAt.After(now) || !inv.RevokedAt.IsZero() ||
		inv.SubmissionState == models.InvitationStateSubmitted ||
		(inv.SubmissionState == "" && !inv.SubmittedAt.IsZero()) ||
		(inv.SubmissionState != "" && !inv.SubmissionState.Valid())
}

func containsQuestion(form models.FormDB, id models.DBID) bool {
	for _, info := range form.QuestionsInfo {
		if info.IDQuestion == id {
			return true
		}
	}
	for _, questions := range form.Sections {
		for _, info := range questions {
			if info.IDQuestion == id {
				return true
			}
		}
	}
	return false
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func validateAnswers(form models.FormDB, answers models.Answers[string]) error {
	if len(answers) == 0 || len(answers) > 200 {
		return fmt.Errorf("invalid answer count")
	}
	required := map[string]bool{}
	for _, info := range form.QuestionsInfo {
		if !info.Optional {
			required[info.IDQuestion.Hex()] = true
		}
	}
	for _, section := range form.Sections {
		for _, info := range section {
			if !info.Optional {
				required[info.IDQuestion.Hex()] = true
			}
		}
	}
	for questionID := range required {
		values, ok := answers[questionID]
		if !ok || len(values) == 0 {
			return fmt.Errorf("required question is unanswered")
		}
		hasNonEmptyValue := false
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				hasNonEmptyValue = true
				break
			}
		}
		if !hasNonEmptyValue {
			return fmt.Errorf("required question is unanswered")
		}
	}

	for questionID, values := range answers {
		questionOID, parseErr := models.ID.ToDB(questionID)
		if parseErr != nil || !containsQuestion(form, questionOID) || len(values) > 50 {
			return fmt.Errorf("invalid answers")
		}
		for _, value := range values {
			if len(value) > 4000 {
				return fmt.Errorf("answer is too long")
			}
		}
		qResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: questionOID}}, models.FormQuestionDB{})
		if qResult.IsErr() {
			return fmt.Errorf("invalid question")
		}
		question := models.InterfaceTo[models.FormQuestionDB](qResult.Value())
		typeResult := configs.DB.FindOne(bson.D{{Key: "_id", Value: question.IDQuestionType}}, models.FormQuestionTypeDB{})
		if typeResult.IsErr() {
			return fmt.Errorf("invalid question type")
		}
		questionType := normalizeQuestionType(models.InterfaceTo[models.FormQuestionTypeDB](typeResult.Value()).Name)
		if (questionType == "abierta" || questionType == "opcion unica" || isTrueFalseQuestionType(questionType)) && len(values) > 1 {
			return fmt.Errorf("too many answers for question type")
		}
		allowed := map[string]bool{}
		for _, option := range question.Options {
			allowed[option.Text] = true
		}
		if (questionType == "opcion unica" || strings.Contains(questionType, "respuesta")) && len(allowed) == 0 {
			return fmt.Errorf("choice question has no configured options")
		}
		for _, value := range values {
			if len(allowed) > 0 && !allowed[value] {
				return fmt.Errorf("answer is not a valid option")
			}
			if isTrueFalseQuestionType(questionType) && value != "Verdadero" && value != "Falso" {
				return fmt.Errorf("invalid boolean answer")
			}
		}
	}
	return nil
}

func normalizeQuestionType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("ó", "o", "ú", "u", "í", "i", "á", "a", "é", "e").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func isTrueFalseQuestionType(value string) bool {
	return strings.Contains(value, "verdadero") && strings.Contains(value, "falso")
}
