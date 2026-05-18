package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	courseID           = "es_001"
	fullScaleUnits     = 500
	lessonsPerUnit     = 100
	exercisesPerLesson = 50

	// Target byte size for grammar_notes and cultural_note (the bulk of each
	// record). Total per-exercise JSON lands around ~4 KB → ~10 GB at scale 1.0.
	grammarNotesTargetChars = 1800
	culturalNoteTargetChars = 1800
)

type Spanish struct {
	Text            string `json:"text"`
	IPA             string `json:"ipa"`
	AudioURI        string `json:"audio_uri"`
	AudioDurationMs int    `json:"audio_duration_ms"`
}

type English struct {
	Text string `json:"text"`
}

type Exercise struct {
	ExerciseID   string   `json:"exercise_id"`
	LessonID     string   `json:"lesson_id"`
	UnitID       string   `json:"unit_id"`
	Type         string   `json:"type"`
	Difficulty   float64  `json:"difficulty"`
	Spanish      Spanish  `json:"spanish"`
	English      English  `json:"english"`
	Alternatives []string `json:"alternatives"`
	Distractors  []string `json:"distractors"`
	Hints        []string `json:"hints"`
	GrammarNotes string   `json:"grammar_notes"`
	CulturalNote string   `json:"cultural_note"`
	VocabRefs    []string `json:"vocab_refs"`
	SkillTags    []string `json:"skill_tags"`
	ContentHash  string   `json:"content_hash"`
}

type Manifest struct {
	CourseID          string    `json:"course_id"`
	Seed              int64     `json:"seed"`
	Scale             float64   `json:"scale"`
	TotalUnits        int       `json:"total_units"`
	TotalLessons      int       `json:"total_lessons"`
	TotalExercises    int       `json:"total_exercises"`
	LessonsPerUnit    int       `json:"lessons_per_unit"`
	ExercisesPerLesson int      `json:"exercises_per_lesson"`
	ExerciseIDPattern string    `json:"exercise_id_pattern"`
	GeneratedAt       time.Time `json:"generated_at"`
	BytesWritten      int64     `json:"bytes_written"`
}

func main() {
	var (
		out         = flag.String("out", "./data", "output directory")
		scale       = flag.Float64("scale", 1.0, "scale factor (1.0 = ~10GB / 500 units, 0.01 = ~100MB / 5 units)")
		seed        = flag.Int64("seed", 42, "base RNG seed (deterministic output for a given seed+scale)")
		concurrency = flag.Int("concurrency", runtime.NumCPU(), "parallel unit workers")
	)
	flag.Parse()

	totalUnits := int(float64(fullScaleUnits) * *scale)
	if totalUnits < 1 {
		totalUnits = 1
	}
	totalLessons := totalUnits * lessonsPerUnit
	totalExercises := totalLessons * exercisesPerLesson

	unitsDir := filepath.Join(*out, "units")
	if err := os.MkdirAll(unitsDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}

	log.Printf("generating: units=%d lessons=%d exercises=%d concurrency=%d seed=%d",
		totalUnits, totalLessons, totalExercises, *concurrency, *seed)

	start := time.Now()
	var bytesWritten atomic.Int64
	var unitsDone atomic.Int32

	// Worker pool over unit indices.
	jobs := make(chan int, totalUnits)
	var wg sync.WaitGroup
	for w := 0; w < *concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for unitIdx := range jobs {
				n, err := writeUnit(unitsDir, unitIdx, *seed)
				if err != nil {
					log.Fatalf("unit %d: %v", unitIdx, err)
				}
				bytesWritten.Add(n)
				done := unitsDone.Add(1)
				if done%10 == 0 || int(done) == totalUnits {
					log.Printf("  %d/%d units (%.2f GB)", done, totalUnits,
						float64(bytesWritten.Load())/(1<<30))
				}
			}
		}()
	}
	for i := 1; i <= totalUnits; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	manifest := Manifest{
		CourseID:           courseID,
		Seed:               *seed,
		Scale:              *scale,
		TotalUnits:         totalUnits,
		TotalLessons:       totalLessons,
		TotalExercises:     totalExercises,
		LessonsPerUnit:     lessonsPerUnit,
		ExercisesPerLesson: exercisesPerLesson,
		ExerciseIDPattern:  "ex_%010d (1-based, contiguous from 1 to total_exercises)",
		GeneratedAt:        time.Now().UTC(),
		BytesWritten:       bytesWritten.Load(),
	}
	f, err := os.Create(filepath.Join(*out, "manifest.json"))
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		log.Fatalf("manifest encode: %v", err)
	}
	f.Close()

	log.Printf("done in %s: %d exercises, %.2f GB written to %s",
		time.Since(start).Round(time.Millisecond),
		totalExercises,
		float64(bytesWritten.Load())/(1<<30),
		*out)
}

func writeUnit(unitsDir string, unitIdx int, baseSeed int64) (int64, error) {
	unitID := fmt.Sprintf("u_%03d", unitIdx)
	path := filepath.Join(unitsDir, unitID+".ndjson")
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 1<<20) // 1 MB write buffer
	defer w.Flush()

	rng := rand.New(rand.NewSource(unitSeed(baseSeed, unitIdx)))

	var written int64
	// Lesson IDs and exercise IDs are global and contiguous.
	firstLesson := (unitIdx-1)*lessonsPerUnit + 1
	firstExercise := (unitIdx-1)*lessonsPerUnit*exercisesPerLesson + 1

	for li := 0; li < lessonsPerUnit; li++ {
		lessonID := fmt.Sprintf("l_%05d", firstLesson+li)
		for ei := 0; ei < exercisesPerLesson; ei++ {
			exID := fmt.Sprintf("ex_%010d", firstExercise+li*exercisesPerLesson+ei)
			ex := generateExercise(rng, exID, lessonID, unitID)
			line, err := encodeWithHash(&ex)
			if err != nil {
				return written, err
			}
			n, err := w.Write(line)
			if err != nil {
				return written, err
			}
			written += int64(n)
		}
	}
	return written, w.Flush()
}

func unitSeed(base int64, unitIdx int) int64 {
	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, base)
	_ = binary.Write(h, binary.LittleEndian, int64(unitIdx))
	sum := h.Sum(nil)
	return int64(binary.LittleEndian.Uint64(sum[:8]))
}

func generateExercise(r *rand.Rand, exID, lessonID, unitID string) Exercise {
	n1, n2 := pickNoun(r), pickNoun(r)
	v := pickVerb(r)
	a := pickAdj(r)
	tmpl := sentenceTemplates[r.Intn(len(sentenceTemplates))]
	es := fillTemplate(tmpl.ES, n1, n2, v, a, true)
	en := fillTemplate(tmpl.EN, n1, n2, v, a, false)

	exType := exerciseTypes[r.Intn(len(exerciseTypes))]
	difficulty := float64(r.Intn(1000)) / 1000.0

	return Exercise{
		ExerciseID: exID,
		LessonID:   lessonID,
		UnitID:     unitID,
		Type:       exType,
		Difficulty: difficulty,
		Spanish: Spanish{
			Text:            es,
			IPA:             pseudoIPA(es),
			AudioURI:        "https://cdn.example.com/audio/es/" + exID + ".opus",
			AudioDurationMs: 1500 + r.Intn(4000),
		},
		English:      English{Text: en},
		Alternatives: nVariants(r, en, 3),
		Distractors:  nVariants(r, en, 3),
		Hints:        []string{makeHint(r, n1, v), makeHint(r, a, n2)},
		GrammarNotes: paragraph(r, grammarStems, n1.ES, v.ES, a.ES, grammarNotesTargetChars),
		CulturalNote: paragraph(r, culturalStems, n1.ES, n2.ES, v.ES, culturalNoteTargetChars),
		VocabRefs:    vocabRefs(r, 3+r.Intn(3)),
		SkillTags:    pickTags(r, 2+r.Intn(2)),
	}
}

func pickNoun(r *rand.Rand) wordPair { return nouns[r.Intn(len(nouns))] }
func pickVerb(r *rand.Rand) wordPair { return verbs[r.Intn(len(verbs))] }
func pickAdj(r *rand.Rand) wordPair  { return adjectives[r.Intn(len(adjectives))] }

func fillTemplate(tmpl string, n1, n2, v, a wordPair, spanish bool) string {
	s := tmpl
	if spanish {
		s = strings.ReplaceAll(s, "{N2}", n2.ES)
		s = strings.ReplaceAll(s, "{N}", n1.ES)
		s = strings.ReplaceAll(s, "{V}", v.ES)
		s = strings.ReplaceAll(s, "{A}", a.ES)
	} else {
		s = strings.ReplaceAll(s, "{N2_EN}", n2.EN)
		s = strings.ReplaceAll(s, "{N_EN}", n1.EN)
		// English verb wordPairs are "to X"; strip "to " when used as base.
		ven := strings.TrimPrefix(v.EN, "to ")
		s = strings.ReplaceAll(s, "{V_EN}", ven)
		s = strings.ReplaceAll(s, "{A_EN}", a.EN)
	}
	return s
}

func pseudoIPA(s string) string {
	repl := strings.NewReplacer(
		"ñ", "ɲ", "ll", "ʎ", "rr", "r", "j", "x", "v", "b",
		"z", "s", "c", "k", "h", "", "qu", "k", "á", "a", "é", "e",
		"í", "i", "ó", "o", "ú", "u",
	)
	return "/" + repl.Replace(strings.ToLower(s)) + "/"
}

func nVariants(r *rand.Rand, base string, n int) []string {
	out := make([]string, n)
	suffixes := []string{
		" (informal)", " (formal)", " (regional: MX)", " (regional: ES)",
		" (alt)", " (literary)", " (colloquial)",
	}
	for i := range out {
		out[i] = base + suffixes[r.Intn(len(suffixes))]
	}
	return out
}

func makeHint(r *rand.Rand, a, b wordPair) string {
	stems := []string{
		"Remember: '%s' usually pairs with '%s' in this context.",
		"Think about the meaning of '%s' before choosing — it relates to '%s'.",
		"The Spanish word '%s' shares a root with '%s'; that's your clue.",
		"Hint: focus on the ending of '%s' to figure out '%s'.",
	}
	return fmt.Sprintf(stems[r.Intn(len(stems))], a.ES, b.ES)
}

func paragraph(r *rand.Rand, stems []string, a, b, c string, minChars int) string {
	var sb strings.Builder
	for sb.Len() < minChars {
		stem := stems[r.Intn(len(stems))]
		// Stems take 2 %s args — most do. Format with available substitutions.
		args := []any{anyOf(r, a, b, c), anyOf(r, a, b, c)}
		line := safeSprintf(stem, args...)
		sb.WriteString(line)
		sb.WriteString(" ")
	}
	return strings.TrimSpace(sb.String())
}

func anyOf(r *rand.Rand, opts ...string) string { return opts[r.Intn(len(opts))] }

// safeSprintf tolerates stems with more or fewer %s than we supply.
func safeSprintf(format string, args ...any) string {
	want := strings.Count(format, "%s")
	for len(args) < want {
		args = append(args, "—")
	}
	if len(args) > want {
		args = args[:want]
	}
	return fmt.Sprintf(format, args...)
}

func vocabRefs(r *rand.Rand, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("v_%04d", 1+r.Intn(2000))
	}
	return out
}

func pickTags(r *rand.Rand, n int) []string {
	out := make([]string, 0, n)
	seen := map[string]bool{}
	for len(out) < n {
		t := skillTags[r.Intn(len(skillTags))]
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// encodeWithHash marshals the exercise twice: once with an empty hash to
// derive the content_hash, once with the hash filled in to produce the
// NDJSON line. Trailing newline included.
func encodeWithHash(ex *Exercise) ([]byte, error) {
	ex.ContentHash = ""
	pre, err := json.Marshal(ex)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(pre)
	ex.ContentHash = base64.RawURLEncoding.EncodeToString(sum[:])[:22]
	final, err := json.Marshal(ex)
	if err != nil {
		return nil, err
	}
	return append(final, '\n'), nil
}
