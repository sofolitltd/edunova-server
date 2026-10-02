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
