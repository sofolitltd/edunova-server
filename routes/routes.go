package routes

import (
	"github.com/gin-gonic/gin"

	"edunova-server/handlers"
	"edunova-server/middleware"
)

func SetupRoutes(r *gin.Engine) {
	r.Use(middleware.CORS())

	r.GET("/health", handlers.Health)

	api := r.Group("/api")
	{
		api.POST("/register", handlers.Register)
		api.POST("/login", handlers.Login)
		api.POST("/otp-login", handlers.OTPLogin)
		api.POST("/otp-login/verify", handlers.OTPLoginVerify)
		api.POST("/send-otp", handlers.SendOTP)
		api.POST("/verify-otp", handlers.VerifyOTP)
		api.POST("/resend-otp", handlers.ResendOTP)

		// Public course/exam endpoints (no auth)
		api.GET("/courses", handlers.PublicGetCourses)
		api.GET("/courses/:id", handlers.PublicGetCourse)
		api.GET("/exams/:id/questions", handlers.PublicGetExamQuestions)
		api.POST("/enrollments", handlers.PublicCreateEnrollment)

		auth := api.Group("")
		auth.Use(middleware.AuthRequired())
		{
			auth.GET("/user", handlers.GetUser)
			auth.PUT("/user/profile", handlers.UpdateUserProfile)
			auth.PUT("/change-password", handlers.ChangePassword)
			auth.GET("/user/enrollments", handlers.GetUserEnrollments)
			auth.GET("/user/dashboard", handlers.GetUserDashboardStats)
			auth.GET("/user/free-courses", handlers.GetUserFreeCourses)
			auth.POST("/device-token", handlers.RegisterDeviceToken)
			auth.POST("/doubts", handlers.StudentSubmitDoubt)
			auth.GET("/doubts", handlers.StudentGetMyDoubts)
			auth.GET("/calendar/upcoming", handlers.UserGetUpcomingEvents)
			auth.GET("/lessons/today", handlers.UserGetTodayLessons)
			auth.GET("/notifications", handlers.UserGetNotifications)
			auth.PUT("/notifications/:id/read", handlers.UserMarkNotificationRead)
			auth.PUT("/notifications/read-all", handlers.UserMarkAllNotificationsRead)
			auth.GET("/live-exams", handlers.UserGetLiveExams)
			auth.POST("/exams/:id/submit", handlers.UserSubmitExamResult)
			auth.GET("/my-exam-results", handlers.UserGetExamResults)
			auth.GET("/results", handlers.UserGetResults)
			auth.GET("/results/summary", handlers.UserGetResultSummary)
			auth.GET("/attendance/my", handlers.UserGetMyAttendance)
			auth.GET("/notes", handlers.UserGetNotes)
			auth.GET("/notes/:id", handlers.UserGetNoteByID)
			auth.GET("/daily-content", handlers.UserGetDailyContent)
			auth.GET("/daily-content/revisions", handlers.UserGetRevisionContent)
			auth.POST("/daily-content/:id/viewed", handlers.UserMarkContentViewed)
			auth.GET("/daily-streak", handlers.UserGetDailyStreak)
			auth.POST("/transitions", handlers.UserSubmitTransition)
			auth.GET("/transitions", handlers.UserGetMyTransitions)
			auth.GET("/my-feedback", handlers.UserGetMyFeedback)

			// Interactive practice
			auth.GET("/practice/flashcards", handlers.UserGetFlashcards)
			auth.POST("/practice/flashcards/review", handlers.UserReviewFlashcard)
			auth.GET("/practice/quiz", handlers.UserGetQuiz)
			auth.POST("/practice/quiz/attempt", handlers.UserSubmitQuizAttempt)
			auth.GET("/practice/sentences", handlers.UserGetSentenceExercises)
			auth.POST("/practice/sentences/attempt", handlers.UserSubmitSentenceAttempt)
			auth.GET("/practice/stats", handlers.UserGetPracticeStats)
		}

		// Public articles (no auth)
		api.GET("/articles", handlers.UserGetArticles)
	}

	admin := r.Group("/api/admin")
	{
		admin.POST("/login", handlers.AdminLogin)

		// Teacher portal auth — teachers are a fully separate identity from
		// admin_users (own table, own JWT), not an admin role.
		admin.POST("/teacher-login", handlers.TeacherLogin)

		teacherSecured := admin.Group("")
		teacherSecured.Use(middleware.TeacherRequired())
		{
			teacherSecured.GET("/teacher-profile", handlers.TeacherGetProfile)
			teacherSecured.PUT("/teacher-profile", handlers.TeacherUpdateProfile)
			teacherSecured.PUT("/teacher-change-password", handlers.TeacherChangePassword)
			teacherSecured.GET("/teacher-batches", handlers.TeacherGetMyBatches)
		}

		secured := admin.Group("")
		secured.Use(middleware.AdminRequired())
		{
			secured.GET("/dashboard", handlers.AdminDashboard)
			secured.GET("/profile", handlers.AdminGetProfile)
			secured.PUT("/profile", handlers.AdminUpdateProfile)
			secured.PUT("/change-password", handlers.AdminChangePassword)
			secured.GET("/users", handlers.AdminGetUsers)
			secured.GET("/users/:id", handlers.AdminGetUserByID)
			secured.PUT("/users/:id/verify", handlers.AdminToggleVerify)
			secured.DELETE("/users/:id", handlers.AdminDeleteUser)
			secured.POST("/courses", handlers.AdminCreateCourse)
			secured.PUT("/courses/:id", handlers.AdminUpdateCourse)
			secured.DELETE("/courses/:id", handlers.AdminDeleteCourse)
			secured.GET("/enrollments", handlers.AdminGetEnrollments)
			secured.POST("/enrollments/direct", handlers.AdminDirectEnroll)
			secured.PUT("/enrollments/:id/status", handlers.AdminUpdateEnrollmentStatus)
			secured.DELETE("/enrollments/:id", handlers.AdminDeleteEnrollment)

			secured.GET("/batches/:id/students", handlers.AdminGetBatchStudents)
			secured.GET("/batches/:id/next-student-id", handlers.AdminNextStudentID)
			secured.GET("/batches/check-student-id", handlers.AdminCheckStudentID)

			// Academic management
			secured.GET("/classes", handlers.AdminGetClasses)
			secured.POST("/classes", handlers.AdminCreateClass)
			secured.PUT("/classes/:id", handlers.AdminUpdateClass)
			secured.DELETE("/classes/:id", handlers.AdminDeleteClass)

			secured.GET("/subjects", handlers.AdminGetSubjects)
			secured.POST("/subjects", handlers.AdminCreateSubject)
			secured.PUT("/subjects/:id", handlers.AdminUpdateSubject)
			secured.DELETE("/subjects/:id", handlers.AdminDeleteSubject)

			secured.GET("/books", handlers.AdminGetBooks)
			secured.POST("/books", handlers.AdminCreateBook)
			secured.PUT("/books/:id", handlers.AdminUpdateBook)
			secured.DELETE("/books/:id", handlers.AdminDeleteBook)

			secured.GET("/chapters", handlers.AdminGetChapters)
			secured.POST("/chapters", handlers.AdminCreateChapter)
			secured.PUT("/chapters/:id", handlers.AdminUpdateChapter)
			secured.DELETE("/chapters/:id", handlers.AdminDeleteChapter)

			secured.GET("/topics", handlers.AdminGetTopics)
			secured.POST("/topics", handlers.AdminCreateTopic)
			secured.PUT("/topics/:id", handlers.AdminUpdateTopic)
			secured.DELETE("/topics/:id", handlers.AdminDeleteTopic)

			// Question bank
			secured.GET("/questions", handlers.AdminGetQuestions)
			secured.GET("/questions/:id", handlers.AdminGetQuestion)
			secured.POST("/questions", handlers.AdminCreateQuestion)
			secured.PUT("/questions/:id", handlers.AdminUpdateQuestion)
			secured.DELETE("/questions/:id", handlers.AdminDeleteQuestion)
			secured.PUT("/questions/:id/status", handlers.AdminUpdateQuestionStatus)
			secured.POST("/questions/check-duplicate", handlers.AdminCheckDuplicate)
			secured.POST("/questions/bulk", handlers.AdminBulkUploadQuestions)
			secured.POST("/questions/fetch-sheet", handlers.AdminFetchGoogleSheet)
			secured.GET("/questions/stats", handlers.AdminGetQuestionBankStats)

			// Academic management bulk import
			secured.POST("/academic-management/bulk", handlers.AdminBulkImportHierarchy)

			// Import history
			secured.GET("/imports", handlers.AdminGetImportHistory)

			// Interactive practice content
			secured.GET("/vocabulary", handlers.AdminGetVocabulary)
			secured.POST("/vocabulary", handlers.AdminCreateVocabulary)
			secured.PUT("/vocabulary/:id", handlers.AdminUpdateVocabulary)
			secured.DELETE("/vocabulary/:id", handlers.AdminDeleteVocabulary)
			secured.GET("/sentence-exercises", handlers.AdminGetSentenceExercises)
			secured.POST("/sentence-exercises", handlers.AdminCreateSentenceExercise)
			secured.PUT("/sentence-exercises/:id", handlers.AdminUpdateSentenceExercise)
			secured.DELETE("/sentence-exercises/:id", handlers.AdminDeleteSentenceExercise)

			// Finance
			secured.GET("/finance/stats", handlers.AdminGetFinanceStats)
			secured.GET("/expenses", handlers.AdminGetExpenses)
			secured.POST("/expenses", handlers.AdminCreateExpense)
			secured.PUT("/expenses/:id", handlers.AdminUpdateExpense)
			secured.DELETE("/expenses/:id", handlers.AdminDeleteExpense)

			// Batches
			secured.GET("/batches", handlers.AdminGetBatches)
			secured.GET("/batches/check-name", handlers.AdminCheckBatchName)
			secured.GET("/batches/check-code", handlers.AdminCheckBatchCode)
			secured.POST("/batches", handlers.AdminCreateBatch)
			secured.PUT("/batches/:id", handlers.AdminUpdateBatch)
			secured.DELETE("/batches/:id", handlers.AdminDeleteBatch)
			secured.GET("/batches/stats", handlers.AdminGetBatchStats)
			secured.GET("/batches/:id/finance", handlers.AdminGetBatchFinance)

			// Attendance
			secured.GET("/batches/:id/attendance/monthly", handlers.AdminGetBatchMonthlyAttendance)

			// Smart Calendar
			secured.GET("/calendar", handlers.AdminGetCalendarEvents)
			secured.POST("/calendar", handlers.AdminCreateCalendarEvent)
			secured.PUT("/calendar/:id", handlers.AdminUpdateCalendarEvent)
			secured.DELETE("/calendar/:id", handlers.AdminDeleteCalendarEvent)

			// Payments
			secured.GET("/payments", handlers.AdminGetPayments)
			secured.POST("/payments", handlers.AdminCreatePayment)
			secured.PUT("/payments/:id/verify", handlers.AdminVerifyPayment)
			secured.PUT("/payments/:id/reject", handlers.AdminRejectPayment)
			secured.DELETE("/payments/:id", handlers.AdminDeletePayment)

			// Articles / Parenting Hub
			secured.GET("/articles", handlers.AdminGetArticles)
			secured.POST("/articles", handlers.AdminCreateArticle)
			secured.PUT("/articles/:id", handlers.AdminUpdateArticle)
			secured.DELETE("/articles/:id", handlers.AdminDeleteArticle)
			secured.PUT("/articles/:id/toggle", handlers.AdminToggleArticlePublish)

			// Notes
			secured.GET("/notes", handlers.AdminGetNotes)
			secured.POST("/notes", handlers.AdminCreateNote)
			secured.PUT("/notes/:id", handlers.AdminUpdateNote)
			secured.DELETE("/notes/:id", handlers.AdminDeleteNote)

			// Transitions
			secured.GET("/transitions", handlers.AdminGetTransitions)

			// Notifications (FCM)
			secured.POST("/notifications/send", handlers.AdminSendNotification)
			secured.GET("/notifications/history", handlers.AdminGetNotificationHistory)
			secured.DELETE("/notifications/:id", handlers.AdminDeleteNotification)
			secured.GET("/notifications/stats", handlers.AdminGetNotificationStats)
			secured.GET("/notifications/devices", handlers.AdminGetNotificationDevices)

			// Teacher management (admin & master_admin) — teachers live in
			// their own table, this just administers those accounts.
			secured.GET("/teachers", handlers.AdminListTeachers)
			secured.POST("/teachers", handlers.AdminCreateTeacher)
			secured.DELETE("/teachers/:id", handlers.AdminDeleteTeacher)
			secured.PUT("/teachers/:id/reset-password", handlers.AdminResetTeacherPassword)

			// Batch <-> teacher assignment
			secured.GET("/batches/:id/teachers", handlers.AdminGetBatchTeachers)
			secured.POST("/batches/:id/teachers", handlers.AdminAssignBatchTeacher)
			secured.DELETE("/batches/:id/teachers/:teacherId", handlers.AdminUnassignBatchTeacher)

			// Admin management (master_admin only)
			master := secured.Group("")
			master.Use(middleware.MasterRequired())
			{
				master.GET("/admins", handlers.AdminListAdmins)
				master.POST("/admins", handlers.AdminCreateAdmin)
				master.PUT("/admins/:id/role", handlers.AdminUpdateAdminRole)
				master.DELETE("/admins/:id", handlers.AdminDeleteAdmin)
				master.PUT("/admins/:id/reset-password", handlers.AdminResetPassword)
			}
		}

		// Shared staff endpoints — reachable by an admin token OR an
		// independent teacher token, since teachers need these for their
		// own portal (attendance, lessons, daily content, exams, results,
		// doubts) without being part of admin_users.
		staff := admin.Group("")
		staff.Use(middleware.StaffRequired())
		{
			staff.GET("/courses", handlers.AdminGetCourses)
			staff.GET("/users/search", handlers.AdminSearchUsers)

			// Attendance
			staff.GET("/students", handlers.AdminGetStudents)
			staff.POST("/attendance", handlers.AdminMarkAttendance)
			staff.GET("/attendance", handlers.AdminGetAttendance)
			staff.GET("/attendance/report", handlers.AdminGetAttendanceReport)
			staff.GET("/holidays", handlers.AdminGetHolidays)
			staff.POST("/holidays", handlers.AdminCreateHoliday)
			staff.DELETE("/holidays/:id", handlers.AdminDeleteHoliday)

			// Doubt Resolution Tracker
			staff.GET("/doubts", handlers.AdminGetDoubts)
			staff.GET("/doubts/stats", handlers.AdminGetDoubtStats)
			staff.PUT("/doubts/:id/resolve", handlers.AdminResolveDoubt)
			staff.PUT("/doubts/:id/close", handlers.AdminCloseDoubt)

			// Lessons
			staff.GET("/lessons", handlers.AdminGetLessons)
			staff.GET("/lessons/today", handlers.AdminGetTodayLessons)
			staff.POST("/lessons", handlers.AdminCreateLesson)
			staff.PUT("/lessons/:id", handlers.AdminUpdateLesson)
			staff.DELETE("/lessons/:id", handlers.AdminDeleteLesson)

			// Daily Content
			staff.GET("/daily-content", handlers.AdminGetDailyContent)
			staff.POST("/daily-content", handlers.AdminCreateDailyContent)
			staff.PUT("/daily-content/:id", handlers.AdminUpdateDailyContent)
			staff.DELETE("/daily-content/:id", handlers.AdminDeleteDailyContent)

			// Exams
			staff.GET("/exams", handlers.AdminGetExams)
			staff.POST("/exams", handlers.AdminCreateExam)
			staff.PUT("/exams/:id", handlers.AdminUpdateExam)
			staff.DELETE("/exams/:id", handlers.AdminDeleteExam)
			staff.GET("/exams/:id/questions", handlers.AdminGetExamQuestions)
			staff.POST("/exams/:id/questions", handlers.AdminAddExamQuestion)
			staff.DELETE("/exams/:id/questions/:qid", handlers.AdminDeleteExamQuestion)
			staff.PUT("/exams/:id/live", handlers.AdminToggleExamLive)

			// Results / Progress
			staff.GET("/results", handlers.AdminGetResults)
			staff.POST("/results", handlers.AdminCreateResult)
			staff.PUT("/results/:id", handlers.AdminUpdateResult)
			staff.DELETE("/results/:id", handlers.AdminDeleteResult)
		}
	}
}
