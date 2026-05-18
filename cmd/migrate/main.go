package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"course-api/config"
	db_pkg "course-api/db"
)

func absPath(p string) (string, error) {
	return filepath.Abs(p)
}

func main() {
	dbURL := flag.String("db", "root:root@tcp(localhost:3306)/course_api", "MySQL DSN")
	scale := flag.Float64("scale", 0.01, "dataset scale factor")
	seed := flag.Int64("seed", 42, "dataset seed")
	generatorDir := flag.String("generator-dir", "./data-generator", "path to data-generator directory")
	dataDir := flag.String("data-dir", "./data", "path to data output directory")
	yes := flag.Bool("yes", false, "skip confirmation prompt before wiping user state")
	skipData := flag.Bool("skip-data", false, "only apply SQL migrations, skip data loading")
	commitEvery := flag.Int("commit-every", 25_000, "commit the data-load tx every N rows")
	flag.Parse()

	migrationsDir := "db/migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "../../db/migrations"
	}

	database := db_pkg.Open(*dbURL, config.CreateViper())
	defer database.Close()

	db_pkg.Migrate(database, migrationsDir)

	if *skipData {
		fmt.Println("migrations applied, skipping data load (--skip-data)")
		return
	}

	existingScale, existingSeed := readMeta(database)
	needRegenerate := existingScale != *scale || existingSeed != *seed

	if needRegenerate && existingScale != 0 {
		fmt.Printf("DATASET CHANGE: was scale=%.4f seed=%d, now scale=%.4f seed=%d\n",
			existingScale, existingSeed, *scale, *seed)
		if !*yes {
			fmt.Print("This will wipe all user state and exercises. Continue? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("aborted")
				os.Exit(1)
			}
		}
		if err := wipeUserState(database); err != nil {
			log.Fatalf("wipe user state: %v", err)
		}
		log.Println("user state wiped")
	} else if !needRegenerate {
		fmt.Printf("dataset unchanged (scale=%.4f seed=%d), nothing to do\n", *scale, *seed)
		return
	}

	absDataDir, err := absPath(*dataDir)
	if err != nil {
		log.Fatalf("resolve data-dir: %v", err)
	}
	absGenDir, err := absPath(*generatorDir)
	if err != nil {
		log.Fatalf("resolve generator-dir: %v", err)
	}

	log.Printf("running generator: scale=%.4f seed=%d", *scale, *seed)
	cmd := exec.Command("go", "run", ".",
		fmt.Sprintf("--scale=%.4f", *scale),
		fmt.Sprintf("--seed=%d", *seed),
		fmt.Sprintf("--out=%s", absDataDir),
	)
	cmd.Dir = absGenDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("generator failed: %v", err)
	}

	manifestPath := filepath.Join(*dataDir, "manifest.json")
	totalExercises, err := readManifestTotal(manifestPath)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}

	log.Printf("loading %d exercises into db (commit every %d rows)...", totalExercises, *commitEvery)
	if err := loadExercises(database, *dataDir, *commitEvery); err != nil {
		log.Fatalf("load exercises: %v", err)
	}

	if err := writeMeta(database, *scale, *seed, totalExercises); err != nil {
		log.Fatalf("write meta: %v", err)
	}

	log.Printf("done: %d exercises loaded", totalExercises)
}

func readMeta(database *sql.DB) (scale float64, seed int64) {
	var scaleStr, seedStr string
	_ = database.QueryRow(`SELECT value FROM meta WHERE meta_key='dataset_scale'`).Scan(&scaleStr)
	_ = database.QueryRow(`SELECT value FROM meta WHERE meta_key='dataset_seed'`).Scan(&seedStr)
	scale, _ = strconv.ParseFloat(scaleStr, 64)
	seed, _ = strconv.ParseInt(seedStr, 10, 64)
	return
}

func wipeUserState(database *sql.DB) error {
	for _, tbl := range []string{"user_progress", "user_completion", "user_daily", "exercises"} {
		if _, err := database.Exec("DELETE FROM " + tbl); err != nil {
			return fmt.Errorf("delete %s: %w", tbl, err)
		}
	}
	return nil
}

func readManifestTotal(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var m struct {
		TotalExercises int64 `json:"total_exercises"`
	}
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return 0, err
	}
	return m.TotalExercises, nil
}

// txLoader streams batches into the DB and commits whenever
// commitEvery rows have been written since the last commit. This
// keeps a single transaction from spanning the entire (potentially
// multi-GB) dataset, which would blow MySQL's max_allowed_packet
// and innodb_log_file_size at scale=0.1 and above.
type txLoader struct {
	db           *sql.DB
	tx           *sql.Tx
	commitEvery  int
	sinceCommit  int
	totalLoaded  int64
	lastLoggedAt int64
}

func newTxLoader(db *sql.DB, commitEvery int) (*txLoader, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	return &txLoader{db: db, tx: tx, commitEvery: commitEvery}, nil
}

// flushBatch executes one REPLACE INTO with the given batch of rows.
func (l *txLoader) flushBatch(batch []rowArgs) error {
	if len(batch) == 0 {
		return nil
	}
	placeholder := strings.Repeat("(?,?,?,?,?),", len(batch))
	query := "REPLACE INTO exercises (exercise_id, canonical_id, unit_id, lesson_id, content) VALUES " +
		placeholder[:len(placeholder)-1]

	args := make([]interface{}, 0, len(batch)*5)
	for _, r := range batch {
		args = append(args, r[0], r[1], r[2], r[3], r[4])
	}
	if _, err := l.tx.Exec(query, args...); err != nil {
		return err
	}

	l.sinceCommit += len(batch)
	l.totalLoaded += int64(len(batch))

	if l.totalLoaded-l.lastLoggedAt >= 5000 {
		log.Printf("  loaded %d exercises...", l.totalLoaded)
		l.lastLoggedAt = l.totalLoaded
	}

	if l.sinceCommit >= l.commitEvery {
		if err := l.tx.Commit(); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		tx, err := l.db.Begin()
		if err != nil {
			return fmt.Errorf("begin next tx: %w", err)
		}
		l.tx = tx
		l.sinceCommit = 0
	}
	return nil
}

// finish flushes any open tx. Safe to call multiple times.
func (l *txLoader) finish() error {
	if l.tx == nil {
		return nil
	}
	err := l.tx.Commit()
	l.tx = nil
	return err
}

// rollback is best-effort; used in error paths.
func (l *txLoader) rollback() {
	if l.tx != nil {
		_ = l.tx.Rollback()
		l.tx = nil
	}
}

func loadExercises(database *sql.DB, dataDir string, commitEvery int) error {
	unitsDir := filepath.Join(dataDir, "units")
	entries, err := os.ReadDir(unitsDir)
	if err != nil {
		return fmt.Errorf("read units dir: %w", err)
	}

	loader, err := newTxLoader(database, commitEvery)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".ndjson" {
			continue
		}

		path := filepath.Join(unitsDir, entry.Name())
		if err := loadFile(loader, path); err != nil {
			loader.rollback()
			return fmt.Errorf("load %s: %w", entry.Name(), err)
		}
	}

	if err := loader.finish(); err != nil {
		return fmt.Errorf("final commit: %w", err)
	}
	log.Printf("  loaded %d exercises total", loader.totalLoaded)
	return nil
}

type exerciseRow struct {
	ExerciseIDStr string `json:"exercise_id"`
	LessonIDStr   string `json:"lesson_id"`
	UnitIDStr     string `json:"unit_id"`
}

// rowArgs holds the 5 column values for one row: exID, canonicalID, unitID, lessonID, content.
type rowArgs [5]interface{}

const batchSize = 500

func loadFile(loader *txLoader, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	batch := make([]rowArgs, 0, batchSize)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var row exerciseRow
		if err := json.Unmarshal(line, &row); err != nil {
			return fmt.Errorf("parse line: %w", err)
		}

		var exID, unitID, lessonID int64
		fmt.Sscanf(row.ExerciseIDStr, "ex_%d", &exID)
		fmt.Sscanf(row.UnitIDStr, "u_%d", &unitID)
		fmt.Sscanf(row.LessonIDStr, "l_%d", &lessonID)

		batch = append(batch, rowArgs{exID, row.ExerciseIDStr, unitID, lessonID, string(line)})

		if len(batch) == batchSize {
			if err := loader.flushBatch(batch); err != nil {
				return fmt.Errorf("flush batch ending at %s: %w", row.ExerciseIDStr, err)
			}
			batch = batch[:0]
		}
	}
	if err := loader.flushBatch(batch); err != nil {
		return fmt.Errorf("flush final batch: %w", err)
	}
	return scanner.Err()
}

func writeMeta(database *sql.DB, scale float64, seed, total int64) error {
	for _, row := range []struct{ k, v string }{
		{"dataset_scale", strconv.FormatFloat(scale, 'f', -1, 64)},
		{"dataset_seed", strconv.FormatInt(seed, 10)},
		{"total_exercises", strconv.FormatInt(total, 10)},
	} {
		if _, err := database.Exec(
			`REPLACE INTO meta (meta_key, value) VALUES (?, ?)`, row.k, row.v,
		); err != nil {
			return fmt.Errorf("write meta %s: %w", row.k, err)
		}
	}
	return nil
}
