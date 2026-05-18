package services_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"course-api/config"
	db_pkg "course-api/db"
	"course-api/repository"
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

func setupBotTestDB(t *testing.T) *sql.DB {
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

// TestBotService_FastSingleDwell verifies that a single completion under the
// minimum dwell threshold flags the user as a bot.
func TestBotService_FastSingleDwell(t *testing.T) {
	db := setupBotTestDB(t)
	repo := repository.New(db)
	bs := services.NewBotService(5_000, 3_000, 100, 10)

	userID := uuid.New().String()
	require.NoError(t, repo.CreateUser(userID, time.Now().Unix()))

	var newStatus string
	err := repo.WithTx(func(tx repository.TxRepo) error {
		start := time.Now().UnixMilli() - 50 // 50ms dwell, well under 5s threshold
		require.NoError(t, tx.RecordStart(userID, 1, 1, start))
		require.NoError(t, tx.RecordEndTime(userID, 1, time.Now().UnixMilli()))
		var err error
		newStatus, err = bs.DetectNewStatus(tx, userID, 1, "real")
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, "bot", newStatus)
}

// TestBotService_LegitDwell verifies that a completion above both thresholds stays real.
func TestBotService_LegitDwell(t *testing.T) {
	db := setupBotTestDB(t)
	repo := repository.New(db)
	bs := services.NewBotService(100, 100, 1000, 10)

	userID := uuid.New().String()
	require.NoError(t, repo.CreateUser(userID, time.Now().Unix()))

	var newStatus string
	err := repo.WithTx(func(tx repository.TxRepo) error {
		start := time.Now().UnixMilli() - 500 // 500ms dwell, above 100ms threshold
		require.NoError(t, tx.RecordStart(userID, 1, 1, start))
		require.NoError(t, tx.RecordEndTime(userID, 1, time.Now().UnixMilli()))
		var err error
		newStatus, err = bs.DetectNewStatus(tx, userID, 1, "real")
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, "real", newStatus)
}

// TestBotService_DailyCap verifies that exceeding the daily lesson cap flags as bot.
func TestBotService_DailyCap(t *testing.T) {
	db := setupBotTestDB(t)
	repo := repository.New(db)
	bs := services.NewBotService(0, 0, 2, 10) // cap at 2 lessons/day

	userID := uuid.New().String()
	require.NoError(t, repo.CreateUser(userID, time.Now().Unix()))

	// Complete 3 lessons; the 3rd should trip the cap.
	for i := int64(1); i <= 3; i++ {
		slot := i % 100
		var newStatus string
		err := repo.WithTx(func(tx repository.TxRepo) error {
			start := time.Now().UnixMilli() - 1000
			require.NoError(t, tx.RecordStart(userID, slot, i, start))
			require.NoError(t, tx.RecordEndTime(userID, slot, time.Now().UnixMilli()))
			var err error
			newStatus, err = bs.DetectNewStatus(tx, userID, slot, "real")
			return err
		})
		require.NoError(t, err)
		if i <= 2 {
			assert.Equal(t, "real", newStatus, "lesson %d should be real", i)
		} else {
			assert.Equal(t, "bot", newStatus, "lesson %d should trip the cap", i)
		}
	}
}

// TestBotService_AlreadyBot verifies the short-circuit: no DB queries when already a bot.
func TestBotService_AlreadyBot(t *testing.T) {
	db := setupBotTestDB(t)
	repo := repository.New(db)
	bs := services.NewBotService(0, 0, 0, 10)

	userID := uuid.New().String()
	require.NoError(t, repo.CreateUser(userID, time.Now().Unix()))

	var newStatus string
	err := repo.WithTx(func(tx repository.TxRepo) error {
		var err error
		// Pass currentStatus="bot" — should return immediately without touching any table.
		newStatus, err = bs.DetectNewStatus(tx, userID, 1, "bot")
		return err
	})
	require.NoError(t, err)
	assert.Equal(t, "bot", newStatus)
}
