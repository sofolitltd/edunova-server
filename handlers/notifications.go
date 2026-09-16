package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
	"edunova-server/services"
)

// buildLinkData builds FCM data payload for deep linking
func buildLinkData(linkType string, linkID int) map[string]string {
	if linkType == "" {
		return nil

	}
	return map[string]string{
		"type": linkType,
		"id":   strconv.Itoa(linkID),
	}
}

// storeNotification saves notification to DB and returns its ID
func storeNotification(ctx context.Context, title, body, target string, targetID, linkID int, linkType string, sentBy int) int {
	var notifID int
	err := database.DB.QueryRow(ctx,
		`INSERT INTO notifications (title, body, target, target_id, link_type, link_id, sent_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		title, body, target, targetID, linkType, linkID, sentBy,
	).Scan(&notifID)
	if err != nil {
		return 0
	}
	return notifID
}

func AdminSendNotification(c *gin.Context) {
	var req models.SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := context.Background()
	adminID := c.GetInt("admin_id")
	linkData := buildLinkData(req.LinkType, req.LinkID)

	// All users → single topic call
	if req.AllUsers {
		if err := services.SendFCMTopicV1WithData("all", req.Title, req.Body, linkData); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send notification"})
			return
		}
		notifID := storeNotification(ctx, req.Title, req.Body, "all", 0, req.LinkID, req.LinkType, adminID)
		c.JSON(http.StatusOK, gin.H{
			"message":         "notification sent to all users",
			"target":          "all",
			"topic":           "all",
			"notification_id": notifID,
		})
		return
	}

	// Specific batch → single topic call
	if req.BatchID > 0 {
		topic := fmt.Sprintf("batch_%d", req.BatchID)
		if err := services.SendFCMTopicV1WithData(topic, req.Title, req.Body, linkData); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send notification"})
			return
		}
		var batchName string
		database.DB.QueryRow(ctx, `SELECT name FROM batches WHERE id=$1`, req.BatchID).Scan(&batchName)
		notifID := storeNotification(ctx, req.Title, req.Body, "batch", req.BatchID, req.LinkID, req.LinkType, adminID)
		c.JSON(http.StatusOK, gin.H{
			"message":         "notification sent to batch",
			"target":          "batch",
			"batch_id":        req.BatchID,
			"batch_name":      batchName,
			"topic":           topic,
			"notification_id": notifID,
		})
		return
	}

	// Specific student → send to their token directly (targeted)
	if req.UserID > 0 {
		tokens := fetchUserTokens(ctx, req.UserID)
		if len(tokens) == 0 {
			topic := fmt.Sprintf("student_%d", req.UserID)
			if err := services.SendFCMTopicV1WithData(topic, req.Title, req.Body, linkData); err != nil {
				c.JSON(http.StatusOK, gin.H{"message": "no devices registered for this user", "total": 0})
				return
			}
			notifID := storeNotification(ctx, req.Title, req.Body, "student", req.UserID, req.LinkID, req.LinkType, adminID)
			c.JSON(http.StatusOK, gin.H{
				"message":         "notification sent via topic",
				"target":          "student",
				"user_id":         req.UserID,
				"topic":           topic,
				"notification_id": notifID,
			})
			return
		}

		success := 0
		failure := 0
		for _, token := range tokens {
			if err := services.SendFCMV1ToTokenWithData(token, req.Title, req.Body, linkData); err != nil {
				failure++
			} else {
				success++
			}
		}
		notifID := storeNotification(ctx, req.Title, req.Body, "student", req.UserID, req.LinkID, req.LinkType, adminID)
		c.JSON(http.StatusOK, gin.H{
			"message":         "notification sent to student",
			"target":          "student",
			"user_id":         req.UserID,
			"total":           len(tokens),
			"success":         success,
			"failure":         failure,
			"notification_id": notifID,
		})
		return
	}

	c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "provide user_id, batch_id, or set all_users to true"})
}

func fetchUserTokens(ctx context.Context, userID int) []string {
	rows, err := database.DB.Query(ctx, `SELECT token FROM device_tokens WHERE user_id=$1`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

// AdminGetNotificationHistory returns all sent notifications
func AdminGetNotificationHistory(c *gin.Context) {
	ctx := context.Background()
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	target := c.Query("target")
	targetID, _ := strconv.Atoi(c.Query("target_id"))

	query := `SELECT n.id, n.title, n.body, n.target, n.target_id, n.link_type, n.link_id,
	                 COALESCE(n.sent_by, 0), n.sent_at
	          FROM notifications n`
	countQuery := `SELECT COUNT(*) FROM notifications n`
	args := []interface{}{}
	if target != "" && targetID > 0 {
		query += ` WHERE n.target = $1 AND n.target_id = $2`
		countQuery += ` WHERE n.target = $1 AND n.target_id = $2`
		args = append(args, target, targetID)
	}
	query += fmt.Sprintf(` ORDER BY n.sent_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := database.DB.Query(ctx, query, append(args, limit, offset)...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch notifications"})
		return
	}
	defer rows.Close()

	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Target, &n.TargetID,
			&n.LinkType, &n.LinkID, &n.SentBy, &n.SentAt); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []models.Notification{}
	}

	var total int
	database.DB.QueryRow(ctx, countQuery, args...).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"total":         total,
		"page":          page,
		"limit":         limit,
	})
}

// AdminDeleteNotification deletes a notification from history
func AdminDeleteNotification(c *gin.Context) {
	id := c.Param("id")
	result, err := database.DB.Exec(context.Background(),
		`DELETE FROM notifications WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete"})
		return
	}
	if rows := result.RowsAffected(); rows == 0 {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "deleted"})
}

// AdminGetNotificationStats returns device counts
func AdminGetNotificationStats(c *gin.Context) {
	ctx := context.Background()
	var totalUsers, totalTokens, androidCount, iosCount, totalSent int

	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&totalUsers)
	database.DB.QueryRow(ctx, `SELECT COUNT(DISTINCT token) FROM device_tokens`).Scan(&totalTokens)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM device_tokens WHERE platform='android'`).Scan(&androidCount)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM device_tokens WHERE platform='ios'`).Scan(&iosCount)
	database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM notifications`).Scan(&totalSent)

	c.JSON(http.StatusOK, gin.H{
		"total_users":   totalUsers,
		"total_devices": totalTokens,
		"android":       androidCount,
		"ios":           iosCount,
		"total_sent":    totalSent,
	})
}

func AdminGetNotificationDevices(c *gin.Context) {
	query := `SELECT dt.id, dt.user_id, COALESCE(u.full_name,''), dt.platform, dt.created_at
		 FROM device_tokens dt
		 LEFT JOIN users u ON dt.user_id = u.id`
	args := []interface{}{}
	if userID := c.Query("user_id"); userID != "" {
		query += ` WHERE dt.user_id = $1`
		args = append(args, userID)
	}
	query += ` ORDER BY dt.created_at DESC`

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch devices"})
		return
	}
	defer rows.Close()

	type DeviceInfo struct {
		ID        int    `json:"id"`
		UserID    int    `json:"user_id"`
		UserName  string `json:"user_name"`
		Platform  string `json:"platform"`
		CreatedAt string `json:"created_at"`
	}

	var devices []DeviceInfo
	for rows.Next() {
		var d DeviceInfo
		if err := rows.Scan(&d.ID, &d.UserID, &d.UserName, &d.Platform, &d.CreatedAt); err != nil {
			continue
		}
		devices = append(devices, d)
	}
	if devices == nil {
		devices = []DeviceInfo{}
	}

	c.JSON(http.StatusOK, gin.H{"devices": devices, "total": len(devices)})
}

// UserGetNotifications returns notifications visible to the user
func UserGetNotifications(c *gin.Context) {
	mobile := c.GetString("mobile")
	ctx := context.Background()

	// Get user ID from mobile
	var userID int
	err := database.DB.QueryRow(ctx, `SELECT id FROM users WHERE mobile=$1`, mobile).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	// Get user's batch IDs for batch-targeted notifications
	var batchIDs []int
	bRows, _ := database.DB.Query(ctx, `SELECT id FROM batches WHERE id IN (SELECT batch_id FROM enrollments WHERE mobile=$1 AND status='approved')`, mobile)
	if bRows != nil {
		defer bRows.Close()
		for bRows.Next() {
			var bid int
			if err := bRows.Scan(&bid); err == nil {
				batchIDs = append(batchIDs, bid)
			}
		}
	}

	// Fetch notifications: all + batch-specific + user-specific
	rows, err := database.DB.Query(ctx,
		`SELECT n.id, n.title, n.body, n.target, n.target_id, n.link_type, n.link_id, n.sent_at,
		        CASE WHEN nr.id IS NOT NULL THEN TRUE ELSE FALSE END as read_by_me
		 FROM notifications n
		 LEFT JOIN notification_reads nr ON nr.notification_id = n.id AND nr.user_id = $1
		 WHERE n.target = 'all'
		    OR (n.target = 'student' AND n.target_id = $1)
		 ORDER BY n.sent_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch notifications"})
		return
	}
	defer rows.Close()

	type NotifResp struct {
		models.Notification
		ReadByMe bool `json:"read_by_me"`
	}

	var notifications []NotifResp
	for rows.Next() {
		var n NotifResp
		if err := rows.Scan(&n.ID, &n.Title, &n.Body, &n.Target, &n.TargetID,
			&n.LinkType, &n.LinkID, &n.SentAt, &n.ReadByMe); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}

	// Add batch notifications
	if len(batchIDs) > 0 {
		batchRows, err := database.DB.Query(ctx,
			`SELECT n.id, n.title, n.body, n.target, n.target_id, n.link_type, n.link_id, n.sent_at,
			        CASE WHEN nr.id IS NOT NULL THEN TRUE ELSE FALSE END as read_by_me
			 FROM notifications n
			 LEFT JOIN notification_reads nr ON nr.notification_id = n.id AND nr.user_id = $1
			 WHERE n.target = 'batch' AND n.target_id = ANY($2)
			 ORDER BY n.sent_at DESC`, userID, batchIDs)
		if err == nil {
			defer batchRows.Close()
			for batchRows.Next() {
				var n NotifResp
				if err := batchRows.Scan(&n.ID, &n.Title, &n.Body, &n.Target, &n.TargetID,
					&n.LinkType, &n.LinkID, &n.SentAt, &n.ReadByMe); err != nil {
					continue
				}
				notifications = append(notifications, n)
			}
		}
	}

	if notifications == nil {
		notifications = []NotifResp{}
	}

	var total int
	database.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications
		 WHERE target='all' OR (target='student' AND target_id=$1)`, userID).Scan(&total)

	c.JSON(http.StatusOK, gin.H{
		"notifications": notifications,
		"total":         total,
		"page":          page,
	})
}

// UserMarkNotificationRead marks a notification as read
func UserMarkNotificationRead(c *gin.Context) {
	notifID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	mobile := c.GetString("mobile")

	var userID int
	err = database.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE mobile=$1`, mobile).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	_, _ = database.DB.Exec(context.Background(),
		`INSERT INTO notification_reads (notification_id, user_id) VALUES ($1, $2)
		 ON CONFLICT (notification_id, user_id) DO NOTHING`, notifID, userID)

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "marked as read"})
}

// UserMarkAllNotificationsRead marks all as read
func UserMarkAllNotificationsRead(c *gin.Context) {
	mobile := c.GetString("mobile")

	var userID int
	err := database.DB.QueryRow(context.Background(), `SELECT id FROM users WHERE mobile=$1`, mobile).Scan(&userID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		return
	}

	database.DB.Exec(context.Background(),
		`INSERT INTO notification_reads (notification_id, user_id)
		 SELECT id, $1 FROM notifications
		 ON CONFLICT (notification_id, user_id) DO NOTHING`, userID)

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "all marked as read"})
}
