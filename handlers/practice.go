package handlers

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"edunova-server/database"
	"edunova-server/models"
)

const (
	pointsPerQuizCorrect  = 10
	pointsPerQuizWrong    = 2
	pointsPerFlashcard    = 3
	pointsPerSentenceDone = 5
	pointsPerLevel        = 100
)

// ---------- Admin: vocabulary ----------

func AdminGetVocabulary(c *gin.Context) {
	classLevel := c.Query("class_level")
	query := `SELECT id, class_level, word, meaning, COALESCE(meaning_bn,''), COALESCE(example_sentence,''), COALESCE(pronunciation,''), created_at FROM vocabulary_words`
	args := []interface{}{}
	if classLevel != "" {
		query += " WHERE class_level = $1"
		args = append(args, classLevel)
	}
	query += " ORDER BY class_level, word"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var words []models.VocabularyWord
	for rows.Next() {
		var w models.VocabularyWord
		if err := rows.Scan(&w.ID, &w.ClassLevel, &w.Word, &w.Meaning, &w.MeaningBn, &w.ExampleSentence, &w.Pronunciation, &w.CreatedAt); err != nil {
			continue
		}
		words = append(words, w)
	}
	if words == nil {
		words = []models.VocabularyWord{}
	}
	c.JSON(http.StatusOK, words)
}

func AdminCreateVocabulary(c *gin.Context) {
	var req models.CreateVocabularyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	adminID := adminIDFromContext(c)
	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO vocabulary_words (class_level, word, meaning, meaning_bn, example_sentence, pronunciation, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		req.ClassLevel, req.Word, req.Meaning, req.MeaningBn, req.ExampleSentence, req.Pronunciation, adminID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create word"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "word added"})
}

func AdminUpdateVocabulary(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateVocabularyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	_, err := database.DB.Exec(context.Background(),
		`UPDATE vocabulary_words SET class_level=$1, word=$2, meaning=$3, meaning_bn=$4, example_sentence=$5, pronunciation=$6 WHERE id=$7`,
		req.ClassLevel, req.Word, req.Meaning, req.MeaningBn, req.ExampleSentence, req.Pronunciation, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update word"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "word updated"})
}

func AdminDeleteVocabulary(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM vocabulary_words WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete word"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "word deleted"})
}

// ---------- Admin: sentence exercises ----------

func AdminGetSentenceExercises(c *gin.Context) {
	classLevel := c.Query("class_level")
	query := `SELECT id, class_level, prompt, COALESCE(sample_answer,''), created_at FROM sentence_exercises`
	args := []interface{}{}
	if classLevel != "" {
		query += " WHERE class_level = $1"
		args = append(args, classLevel)
	}
	query += " ORDER BY class_level, prompt"

	rows, err := database.DB.Query(context.Background(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	var exercises []models.SentenceExercise
	for rows.Next() {
		var e models.SentenceExercise
		if err := rows.Scan(&e.ID, &e.ClassLevel, &e.Prompt, &e.SampleAnswer, &e.CreatedAt); err != nil {
			continue
		}
		exercises = append(exercises, e)
	}
	if exercises == nil {
		exercises = []models.SentenceExercise{}
	}
	c.JSON(http.StatusOK, exercises)
}

func AdminCreateSentenceExercise(c *gin.Context) {
	var req models.CreateSentenceExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	adminID := adminIDFromContext(c)
	var id int
	err := database.DB.QueryRow(context.Background(),
		`INSERT INTO sentence_exercises (class_level, prompt, sample_answer, created_by) VALUES ($1,$2,$3,$4) RETURNING id`,
		req.ClassLevel, req.Prompt, req.SampleAnswer, adminID,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create exercise"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "exercise added"})
}

func AdminUpdateSentenceExercise(c *gin.Context) {
	id := c.Param("id")
	var req models.CreateSentenceExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	_, err := database.DB.Exec(context.Background(),
		`UPDATE sentence_exercises SET class_level=$1, prompt=$2, sample_answer=$3 WHERE id=$4`,
		req.ClassLevel, req.Prompt, req.SampleAnswer, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update exercise"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "exercise updated"})
}

func AdminDeleteSentenceExercise(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec(context.Background(), `DELETE FROM sentence_exercises WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete exercise"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "exercise deleted"})
}

func adminIDFromContext(c *gin.Context) int {
	adminID, _ := c.Get("admin_id")
	if id, ok := adminID.(int); ok {
		return id
	}
	return 0
}

// ---------- Student: flashcards ----------

// studentClassLevel resolves the logged-in user's class, honoring an
// explicit ?class_level= override when provided.
func studentClassLevel(c *gin.Context, userID int) string {
	if cl := c.Query("class_level"); cl != "" {
		return cl
	}
	var classLevel string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COALESCE(student_class,'') FROM users WHERE id = $1`, userID).Scan(&classLevel)
	return classLevel
}

func UserGetFlashcards(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	classLevel := studentClassLevel(c, userID)

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, class_level, word, meaning, COALESCE(meaning_bn,''), COALESCE(example_sentence,''), COALESCE(pronunciation,''), created_at
		 FROM vocabulary_words WHERE class_level = $1 ORDER BY RANDOM()`, classLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	words := []models.VocabularyWord{}
	for rows.Next() {
		var w models.VocabularyWord
		if err := rows.Scan(&w.ID, &w.ClassLevel, &w.Word, &w.Meaning, &w.MeaningBn, &w.ExampleSentence, &w.Pronunciation, &w.CreatedAt); err != nil {
			continue
		}
		words = append(words, w)
	}
	c.JSON(http.StatusOK, words)
}

func UserReviewFlashcard(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	var req models.FlashcardReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	_, err := database.DB.Exec(context.Background(),
		`INSERT INTO practice_attempts (user_id, activity_type, item_id, points_earned) VALUES ($1,'flashcard',$2,$3)`,
		userID, req.WordID, pointsPerFlashcard)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to record review"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"points_earned": pointsPerFlashcard})
}

// ---------- Student: vocabulary quiz ----------

func UserGetQuiz(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	classLevel := studentClassLevel(c, userID)
	count := 5
	if v, err := strconv.Atoi(c.Query("count")); err == nil && v > 0 && v <= 20 {
		count = v
	}

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, word, meaning FROM vocabulary_words WHERE class_level = $1`, classLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	type word struct {
		id      int
		word    string
		meaning string
	}
	var all []word
	for rows.Next() {
		var w word
		if err := rows.Scan(&w.id, &w.word, &w.meaning); err == nil {
			all = append(all, w)
		}
	}
	rows.Close()

	if len(all) < 4 {
		c.JSON(http.StatusOK, []models.QuizQuestion{})
		return
	}

	rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if count > len(all) {
		count = len(all)
	}

	questions := make([]models.QuizQuestion, 0, count)
	for i := 0; i < count; i++ {
		correct := all[i]
		options := []string{correct.meaning}
		// pick 3 distinct distractor meanings from the rest of the pool
		pool := make([]word, 0, len(all)-1)
		for j, w := range all {
			if j != i {
				pool = append(pool, w)
			}
		}
		rand.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })
		for _, w := range pool {
			if len(options) >= 4 {
				break
			}
			options = append(options, w.meaning)
		}
		rand.Shuffle(len(options), func(a, b int) { options[a], options[b] = options[b], options[a] })
		questions = append(questions, models.QuizQuestion{WordID: correct.id, Word: correct.word, Options: options})
	}

	c.JSON(http.StatusOK, questions)
}

func UserSubmitQuizAttempt(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	var req models.QuizAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var correctMeaning string
	err := database.DB.QueryRow(context.Background(),
		`SELECT meaning FROM vocabulary_words WHERE id=$1`, req.WordID).Scan(&correctMeaning)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "word not found"})
		return
	}

	isCorrect := req.SelectedText == correctMeaning
	points := pointsPerQuizWrong
	if isCorrect {
		points = pointsPerQuizCorrect
	}

	_, err = database.DB.Exec(context.Background(),
		`INSERT INTO practice_attempts (user_id, activity_type, item_id, is_correct, points_earned) VALUES ($1,'vocabulary_quiz',$2,$3,$4)`,
		userID, req.WordID, isCorrect, points)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to record attempt"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"correct":         isCorrect,
		"correct_meaning": correctMeaning,
		"points_earned":   points,
	})
}

// ---------- Student: sentence exercises ----------

func UserGetSentenceExercises(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	classLevel := studentClassLevel(c, userID)

	rows, err := database.DB.Query(context.Background(),
		`SELECT id, class_level, prompt, COALESCE(sample_answer,''), created_at
		 FROM sentence_exercises WHERE class_level = $1 ORDER BY RANDOM()`, classLevel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "database error"})
		return
	}
	defer rows.Close()

	exercises := []models.SentenceExercise{}
	for rows.Next() {
		var e models.SentenceExercise
		if err := rows.Scan(&e.ID, &e.ClassLevel, &e.Prompt, &e.SampleAnswer, &e.CreatedAt); err != nil {
			continue
		}
		exercises = append(exercises, e)
	}
	c.JSON(http.StatusOK, exercises)
}

func UserSubmitSentenceAttempt(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}
	var req models.SentenceAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	var sampleAnswer string
	_ = database.DB.QueryRow(context.Background(),
		`SELECT COALESCE(sample_answer,'') FROM sentence_exercises WHERE id=$1`, req.ExerciseID).Scan(&sampleAnswer)

	// Sentence quality isn't machine-graded — this is a participation
	// activity, scored for attempting it; the sample answer is returned so
	// the student can self-check.
	_, err := database.DB.Exec(context.Background(),
		`INSERT INTO practice_attempts (user_id, activity_type, item_id, points_earned) VALUES ($1,'sentence',$2,$3)`,
		userID, req.ExerciseID, pointsPerSentenceDone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to record attempt"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sample_answer": sampleAnswer,
		"points_earned": pointsPerSentenceDone,
	})
}

// ---------- Student: practice stats (points, level, streak) ----------

func UserGetPracticeStats(c *gin.Context) {
	userIDRaw, _ := c.Get("user_id")
	userID, ok := userIDRaw.(int)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "user not found"})
		return
	}

	ctx := context.Background()
	var totalPoints, quizAttempts, quizCorrect, flashcardsSeen, sentencesDone int
	_ = database.DB.QueryRow(ctx, `SELECT COALESCE(SUM(points_earned),0) FROM practice_attempts WHERE user_id=$1`, userID).Scan(&totalPoints)
	_ = database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM practice_attempts WHERE user_id=$1 AND activity_type='vocabulary_quiz'`, userID).Scan(&quizAttempts)
	_ = database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM practice_attempts WHERE user_id=$1 AND activity_type='vocabulary_quiz' AND is_correct=TRUE`, userID).Scan(&quizCorrect)
	_ = database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM practice_attempts WHERE user_id=$1 AND activity_type='flashcard'`, userID).Scan(&flashcardsSeen)
	_ = database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM practice_attempts WHERE user_id=$1 AND activity_type='sentence'`, userID).Scan(&sentencesDone)

	current, longest := computeStreak(ctx, userID)

	c.JSON(http.StatusOK, models.PracticeStats{
		TotalPoints:    totalPoints,
		Level:          totalPoints/pointsPerLevel + 1,
		PointsToNext:   pointsPerLevel - (totalPoints % pointsPerLevel),
		CurrentStreak:  current,
		LongestStreak:  longest,
		QuizAttempts:   quizAttempts,
		QuizCorrect:    quizCorrect,
		FlashcardsSeen: flashcardsSeen,
		SentencesDone:  sentencesDone,
	})
}

// computeStreak derives a real consecutive-day streak from the distinct
// calendar days practice_attempts rows exist for — unlike daily_content's
// "streak" (actually a lifetime distinct-item count), this walks backward
// day by day from today so it actually breaks when a day is missed.
func computeStreak(ctx context.Context, userID int) (current int, longest int) {
	rows, err := database.DB.Query(ctx,
		`SELECT DISTINCT DATE(created_at) FROM practice_attempts WHERE user_id=$1 ORDER BY DATE(created_at) DESC`, userID)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var d time.Time
		if rows.Scan(&d) == nil {
			days = append(days, d)
		}
	}
	if len(days) == 0 {
		return 0, 0
	}

	today := time.Now().Truncate(24 * time.Hour)
	dayIndex := map[string]bool{}
	for _, d := range days {
		dayIndex[d.Format("2006-01-02")] = true
	}

	// Current streak: walk backward from today (or yesterday, if nothing
	// logged yet today) until a gap is found.
	cursor := today
	if !dayIndex[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for dayIndex[cursor.Format("2006-01-02")] {
		current++
		cursor = cursor.AddDate(0, 0, -1)
	}

	// Longest streak: scan the full distinct-day set for the longest
	// consecutive run.
	run := 1
	longest = 1
	for i := 1; i < len(days); i++ {
		if days[i-1].Sub(days[i]) == 24*time.Hour {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 1
		}
	}
	if len(days) == 1 {
		longest = 1
	}

	return current, longest
}
