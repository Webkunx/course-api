package integration_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"course-api/config"
	db_pkg "course-api/db"
	"course-api/entities"
	"course-api/repository"
	"course-api/server"
	"course-api/services"
)

func migrationsPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "db", "migrations")
}

func rootDSN() string {
	if dsn := os.Getenv("TEST_MYSQL_DSN"); dsn != "" {
		return dsn
	}
	return "root:root@tcp(localhost:3306)/"
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	root := rootDSN()

	rootDB, err := sql.Open("mysql", root)
	if err != nil {
		t.Skipf("MySQL not available: %v", err)
	}
	if err := rootDB.Ping(); err != nil {
		rootDB.Close()
		t.Skipf("MySQL not available: %v", err)
	}

	dbName := "test_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	if _, err := rootDB.Exec("CREATE DATABASE " + dbName); err != nil {
		rootDB.Close()
		t.Fatalf("create test database: %v", err)
	}
	rootDB.Close()

	dsn := strings.TrimRight(root, "/") + "/" + dbName
	database := db_pkg.Open(dsn, config.CreateViper())

	db_pkg.Migrate(database, migrationsPath())

	t.Cleanup(func() {
		database.Close()
		rootDB2, _ := sql.Open("mysql", root)
		rootDB2.Exec("DROP DATABASE " + dbName)
		rootDB2.Close()
	})
	return database
}

func setupApp(t *testing.T, totalExercises int, cfg map[string]interface{}) (*fiber.App, *sql.DB) {
	t.Helper()

	database := setupTestDB(t)
	require.NoError(t, insertMeta(database, totalExercises))
	require.NoError(t, insertFixtureExercises(database, totalExercises))

	v := viper.New()
	v.SetDefault(config.MIN_SINGLE_DWELL_MS, 0)
	v.SetDefault(config.MIN_AVG_DWELL_MS, 0)
	v.SetDefault(config.MAX_DAILY_LESSONS, 1000)
	v.SetDefault(config.DWELL_WINDOW, 10)
	for k, val := range cfg {
		v.Set(k, val)
	}

	repo := repository.New(database)
	cs := services.NewCourseService(repo, v)

	app := fiber.New()
	app.Use(recover.New())
	server.AddRoutes(app, cs, repo)

	return app, database
}

func insertMeta(database *sql.DB, totalExercises int) error {
	for _, row := range []struct{ k, v string }{
		{"dataset_scale", "0.01"},
		{"dataset_seed", "42"},
		{"total_exercises", fmt.Sprintf("%d", totalExercises)},
	} {
		if _, err := database.Exec(`REPLACE INTO meta (meta_key, value) VALUES (?, ?)`, row.k, row.v); err != nil {
			return err
		}
	}
	return nil
}

func insertFixtureExercises(database *sql.DB, count int) error {
	for i := 1; i <= count; i++ {
		jsonData := makeExerciseJSON(i)
		unitID := (i-1)/(100*50) + 1
		lessonID := (i-1)/50 + 1
		canonicalID := fmt.Sprintf("ex_%010d", i)
		if _, err := database.Exec(
			`REPLACE INTO exercises (exercise_id, canonical_id, unit_id, lesson_id, content) VALUES (?, ?, ?, ?, ?)`,
			i, canonicalID, unitID, lessonID, string(jsonData),
		); err != nil {
			return fmt.Errorf("insert exercise %d: %w", i, err)
		}
	}
	return nil
}

// makeExerciseJSON creates deterministic JSON with a valid content_hash.
// Field order matches entities.Exercise exactly — required for hash correctness.
func makeExerciseJSON(id int) []byte {
	ex := &entities.Exercise{
		ExerciseID:   fmt.Sprintf("ex_%010d", id),
		LessonID:     fmt.Sprintf("l_%05d", (id-1)/50+1),
		UnitID:       fmt.Sprintf("u_%03d", (id-1)/(50*100)+1),
		Type:         "translate",
		Difficulty:   0.5,
		Spanish:      entities.Spanish{Text: "hola", IPA: "/hola/", AudioURI: "", AudioDurationMs: 1500},
		English:      entities.English{Text: "hello"},
		Alternatives: []string{},
		Distractors:  []string{},
		Hints:        []string{},
		GrammarNotes: "grammar",
		CulturalNote: "cultural",
		VocabRefs:    []string{},
		SkillTags:    []string{},
	}
	pre, _ := json.Marshal(ex)
	sum := sha256.Sum256(pre)
	ex.ContentHash = base64.RawURLEncoding.EncodeToString(sum[:])[:22]
	final, _ := json.Marshal(ex)
	return final
}

func register(t *testing.T, app *fiber.App) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var body struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.NotEmpty(t, body.AccessToken)
	return body.AccessToken
}

func next(t *testing.T, app *fiber.App, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/next", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	return resp
}

func nextExercise(t *testing.T, app *fiber.App, token string) map[string]interface{} {
	t.Helper()
	resp := next(t, app, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var ex map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &ex))
	return ex
}

func complete(t *testing.T, app *fiber.App, token string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/complete", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	return resp
}

// TestHappyPath: register → next → complete → next returns next exercise.
func TestHappyPath(t *testing.T) {
	app, _ := setupApp(t, 20, nil)
	token := register(t, app)

	ex1 := nextExercise(t, app, token)
	assert.Equal(t, "ex_0000000001", ex1["exercise_id"])

	resp := complete(t, app, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	ex2 := nextExercise(t, app, token)
	assert.Equal(t, "ex_0000000002", ex2["exercise_id"])
}

// TestNextIdempotent: calling /next twice without /complete returns the same exercise.
func TestNextIdempotent(t *testing.T) {
	app, _ := setupApp(t, 20, nil)
	token := register(t, app)

	ex1 := nextExercise(t, app, token)
	ex2 := nextExercise(t, app, token)
	assert.Equal(t, ex1["exercise_id"], ex2["exercise_id"])
}

// TestCompleteIdempotent: second /complete is a no-op and daily count stays 1.
func TestCompleteIdempotent(t *testing.T) {
	app, database := setupApp(t, 20, nil)
	token := register(t, app)

	nextExercise(t, app, token)

	resp1 := complete(t, app, token)
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	resp2 := complete(t, app, token)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	today := time.Now().UTC().Format("2006-01-02")
	var count int64
	require.NoError(t, database.QueryRow(`SELECT completed FROM user_daily WHERE user_id=? AND day=?`, token, today).Scan(&count))
	assert.Equal(t, int64(1), count)
}

// TestBotFlagBySingleDwell: MIN_SINGLE_DWELL_MS very high → any completion flags as bot.
func TestBotFlagBySingleDwell(t *testing.T) {
	app, database := setupApp(t, 20, map[string]interface{}{
		config.MIN_SINGLE_DWELL_MS: 999_999_999,
	})
	token := register(t, app)

	nextExercise(t, app, token)
	resp := complete(t, app, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var status string
	require.NoError(t, database.QueryRow(`SELECT status FROM user_progress WHERE user_id=?`, token).Scan(&status))
	assert.Equal(t, "bot", status)

	var cursorBefore int64
	require.NoError(t, database.QueryRow("SELECT `cursor` FROM user_progress WHERE user_id=?", token).Scan(&cursorBefore))

	resp2 := next(t, app, token)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var cursorAfter int64
	require.NoError(t, database.QueryRow("SELECT `cursor` FROM user_progress WHERE user_id=?", token).Scan(&cursorAfter))
	assert.Equal(t, cursorBefore, cursorAfter, "bot cursor must not advance")
}

// TestBotFlagByDailyCap: MAX_DAILY_LESSONS=3, 4th cycle flags as bot.
func TestBotFlagByDailyCap(t *testing.T) {
	app, database := setupApp(t, 20, map[string]interface{}{
		config.MAX_DAILY_LESSONS: 3,
	})
	token := register(t, app)

	for i := 0; i < 4; i++ {
		nextExercise(t, app, token)
		require.Equal(t, http.StatusOK, complete(t, app, token).StatusCode)
	}

	var status string
	require.NoError(t, database.QueryRow(`SELECT status FROM user_progress WHERE user_id=?`, token).Scan(&status))
	assert.Equal(t, "bot", status)
}

// TestRingBufferOverwrite: 105 cycles → user_completion has exactly 100 rows.
func TestRingBufferOverwrite(t *testing.T) {
	app, database := setupApp(t, 120, nil)
	token := register(t, app)

	for i := 0; i < 105; i++ {
		nextExercise(t, app, token)
		require.Equal(t, http.StatusOK, complete(t, app, token).StatusCode)
	}

	var count int
	require.NoError(t, database.QueryRow(`SELECT COUNT(*) FROM user_completion WHERE user_id=?`, token).Scan(&count))
	assert.Equal(t, 100, count)
}

// TestUnauthorized: missing Bearer → 401.
func TestUnauthorized(t *testing.T) {
	app, _ := setupApp(t, 20, nil)

	for _, path := range []string{"/next", "/complete"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		resp, err := app.Test(req, 5000)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, path)
	}
}

// TestContentHashRoundTrip: verify content_hash survives the storage round-trip.
func TestContentHashRoundTrip(t *testing.T) {
	app, _ := setupApp(t, 20, nil)
	token := register(t, app)

	resp := next(t, app, token)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var ex entities.Exercise
	require.NoError(t, json.Unmarshal(body, &ex))
	require.NotEmpty(t, ex.ContentHash)

	saved := ex.ContentHash
	ex.ContentHash = ""
	pre, _ := json.Marshal(&ex)
	sum := sha256.Sum256(pre)
	expected := base64.RawURLEncoding.EncodeToString(sum[:])[:22]
	assert.Equal(t, expected, saved)
}

// TestCourseComplete: 3-exercise fixture; 4th /next → course_complete.
func TestCourseComplete(t *testing.T) {
	app, database := setupApp(t, 3, nil)
	token := register(t, app)

	for i := 0; i < 3; i++ {
		nextExercise(t, app, token)
		require.Equal(t, http.StatusOK, complete(t, app, token).StatusCode)
	}

	resp := next(t, app, token)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var payload map[string]interface{}
	json.Unmarshal(body, &payload)
	assert.Equal(t, true, payload["course_complete"])

	var cursor int64
	require.NoError(t, database.QueryRow("SELECT `cursor` FROM user_progress WHERE user_id=?", token).Scan(&cursor))
	assert.Equal(t, int64(3), cursor)

	_ = bytes.NewReader(nil)
}
