package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

func AdminGetSMSTemplates(c *gin.Context) {
	rows, err := database.DB.Query(context.Background(),
		`SELECT id, name, body, COALESCE(created_by, 0), created_at, updated_at
		 FROM sms_templates ORDER BY name`,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var templates []models.SMSTemplate
	for rows.Next() {
		var t models.SMSTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Body, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []models.SMSTemplate{}
	}

	c.JSON(http.StatusOK, templates)
}

func AdminCreateSMSTemplate(c *gin.Context) {
	var req models.CreateSMSTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	adminID := c.GetInt("admin_id")

	var t models.SMSTemplate
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO sms_templates (name, body, created_by)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, body, COALESCE(created_by, 0), created_at, updated_at`,
		req.Name, req.Body, adminID,
	).Scan(&t.ID, &t.Name, &t.Body, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create template"})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func AdminUpdateSMSTemplate(c *gin.Context) {
	id := c.Param("id")

	var req models.CreateSMSTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var t models.SMSTemplate
	err := database.DB.QueryRow(context.Background(),
		`UPDATE sms_templates SET name=$1, body=$2, updated_at=NOW()
		 WHERE id=$3
		 RETURNING id, name, body, COALESCE(created_by, 0), created_at, updated_at`,
		req.Name, req.Body, id,
	).Scan(&t.ID, &t.Name, &t.Body, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update template"})
		return
	}
	c.JSON(http.StatusOK, t)
}

func AdminDeleteSMSTemplate(c *gin.Context) {
	id := c.Param("id")
	tag, err := database.DB.Exec(context.Background(), `DELETE FROM sms_templates WHERE id = $1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "template not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "template deleted"})
}

type smsRecipient struct {
	mobile string
	name   string
	class  string
}

func substituteTemplate(body string, r smsRecipient) string {
	replacer := strings.NewReplacer(
		"{{name}}", r.name,
		"{{class}}", r.class,
		"{{mobile}}", r.mobile,
	)
	return replacer.Replace(body)
}

func AdminSendSMS(c *gin.Context) {
	var req models.SendSMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	messageBody := req.Message
	if req.TemplateID != nil {
		var tmplBody string
		err := database.DB.QueryRow(context.Background(),
			`SELECT body FROM sms_templates WHERE id = $1`, *req.TemplateID,
		).Scan(&tmplBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "template not found"})
			return
		}
		if messageBody == "" {
			messageBody = tmplBody
		}
	}

	if strings.TrimSpace(messageBody) == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "message is required"})
		return
	}

	var recipients []smsRecipient

	if len(req.StudentIDs) > 0 {
		rows, err := database.DB.Query(context.Background(),
			`SELECT full_name, mobile, COALESCE(notification_mobile,''), COALESCE(student_class,'')
			 FROM users WHERE id = ANY($1)`,
			req.StudentIDs,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
			return
		}
		for rows.Next() {
			var fullName, mobile, notificationMobile, studentClass string
			if err := rows.Scan(&fullName, &mobile, &notificationMobile, &studentClass); err != nil {
				continue
			}
			target := notificationMobile
			if target == "" {
				target = mobile
			}
			recipients = append(recipients, smsRecipient{mobile: target, name: fullName, class: studentClass})
		}
		rows.Close()
	}

	for _, m := range req.Mobiles {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		recipients = append(recipients, smsRecipient{mobile: m})
	}

	if len(recipients) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "no recipients specified"})
		return
	}

	resp := models.SendSMSResponse{Failed: []models.SendSMSFailure{}}
	for _, r := range recipients {
		text := messageBody
		if r.name != "" || r.class != "" {
			text = substituteTemplate(messageBody, r)
		}
		if err := services.SendSMS(r.mobile, text); err != nil {
			resp.Failed = append(resp.Failed, models.SendSMSFailure{Mobile: r.mobile, Error: err.Error()})
			continue
		}
		resp.Sent++
	}

	c.JSON(http.StatusOK, resp)
}

// guardianSMSNumber is where result SMS go: the account's dedicated
// notification number, else the father's, else the account's own mobile.
const guardianSMSNumber = `COALESCE(NULLIF(notification_mobile,''), NULLIF(father_mobile,''), mobile)`

// UserGetSMSOptIn reports whether result SMS are enabled for the caller and
// the number they would go to.
func UserGetSMSOptIn(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var enabled bool
	var mobile string
	err := database.DB.QueryRow(context.Background(),
		`SELECT sms_opt_in, `+guardianSMSNumber+` FROM users WHERE id = $1`, userID).Scan(&enabled, &mobile)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": enabled, "mobile": mobile})
}

// UserSetSMSOptIn lets the guardian turn result SMS on or off.
func UserSetSMSOptIn(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	if _, err := database.DB.Exec(context.Background(),
		`UPDATE users SET sms_opt_in = $1 WHERE id = $2`, req.Enabled, userID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

// sendResultSMS texts the guardian about a new result, at most once per
// student per day (the day's slot is claimed atomically, and released if the
// send fails) so SMS cost stays bounded; push covers everything else.
func sendResultSMS(ctx context.Context, userID int, text string) {
	var mobile string
	err := database.DB.QueryRow(ctx,
		`UPDATE users SET last_result_sms_on = CURRENT_DATE
		 WHERE id = $1 AND sms_opt_in AND (last_result_sms_on IS NULL OR last_result_sms_on < CURRENT_DATE)
		 RETURNING `+guardianSMSNumber, userID).Scan(&mobile)
	if err != nil {
		return
	}
	if err := services.SendSMS(mobile, text); err != nil {
		_, _ = database.DB.Exec(ctx, `UPDATE users SET last_result_sms_on = NULL WHERE id = $1`, userID)
	}
}
