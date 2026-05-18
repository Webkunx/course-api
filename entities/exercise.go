package entities

// Exercise mirrors the generator's struct exactly — field order matters for content_hash.
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

type Spanish struct {
	Text            string `json:"text"`
	IPA             string `json:"ipa"`
	AudioURI        string `json:"audio_uri"`
	AudioDurationMs int    `json:"audio_duration_ms"`
}

type English struct {
	Text string `json:"text"`
}
