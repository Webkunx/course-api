# Implementation Plan — Anti-Scraping Course API

## 1. Problem framing

Serve a corpus of ~2.5M contiguously-numbered exercises (`ex_0000000001`..`ex_<N>`) such that:
- A legitimate learner gets a great experience (P95 < 400 ms on a content fetch).
- An attacker with real paid accounts, instrumented clients, residential proxies, and an LLM-driven scraper, running for 1 hour on 1 machine, recovers as few **unique** `exercise_id` values as possible.

Evaluation reduces to two numbers:
- **Capture score** = `unique_ids_recovered / total_ids_deployed` (lower is better).
- **P95 latency** on legitimate requests must stay under 400 ms.

## 2. Threat model

| Attacker capability | Implication |
|---|---|
| Real paid/trial accounts | Auth alone is worthless; defenses must apply to authenticated users |
| Instrumented browsers / rooted phones | Client-side defenses (DRM, obfuscation) are not a defense |
| LLM-driven adaptive scraper | Patterns will be discovered if exposed |
| Multiple accounts and IPs scaled horizontally | Per-IP rate limits are weak; per-account budgets matter |
| 1 hour wall-clock, 1 machine, no cloud horizontal scale | Defender's math wins if per-account throughput is hard-capped |
| Contiguous `exercise_id` values | IDs are guessable — any ID-lookup endpoint loses immediately |

## 3. Core insight that shaped the design

`exercise_id` is contiguous. If the API exposes anything like `GET /exercise/{id}` for arbitrary IDs, the attacker wins by enumeration regardless of any other defense. **The whole architecture pivots on: never let a client request an exercise by ID. The server alone decides what to return, based on per-user state.**

Combined with: a real learner progresses through a course linearly at human speed. So the server forces that linearity and enforces human timing.

A consequence that makes this design powerful: **multiple fresh accounts collapse to identical sequences**, because all start at cursor=1 and the state machine forbids skip-ahead. Therefore parallel accounts give the attacker zero speedup in the 1-hour eval window.

## 4. Defenses chosen

| Defense | Purpose |
|---|---|
| No ID-lookup endpoint; only `/next` and `/complete` | Prevents enumeration of the contiguous ID space |
| Server-side progression state machine (per-user cursor, no skip-ahead) | Forces extraction to follow a linear, human-paced path |
| `/next` is idempotent; `/complete` is the only state mutation | Safe retries on flaky networks; bot-detection runs in one place |
| Single in-flight exercise per user (`is_active` guard) | Kills intra-account parallelism |
| Min average dwell time on `/complete` (rolling window, last 10) | Detects bot pacing without being polluted by a single human-paced burst |
| Hard floor on single dwell | Catches the "fake one slow lesson then sprint" averaging attack |
| Daily lesson cap per account | Hard ceiling on per-account daily capture |
| Shadow-throttle: flagged users get random exercise from `[1, max(cursor, 1)]` (cursor stays put) | Real exercises (so `content_hash` validates), but unique-count is bounded by `max(cursor, 1)`. Attacker can't distinguish from normal serving |
| Raw UUID v4 bearer token | Simple, sufficient for the threat model (see §13 for rationale) |

## 5. Defenses considered and rejected

| Defense | Rejected because |
|---|---|
| Per-IP register rate limit | Parallel accounts collapse to same sequence — mass account creation gives the attacker zero exfiltration advantage during the 1-hour window. Keeping a very loose 100/min cap **only** to prevent DoS / SQLite bloat in operational hygiene terms — explicitly not labeled as anti-scraping. |
| PoW on `/register` or `/next` | State machine + budget already cap throughput; `pow-buster` (~80 MH/s on €14 VPS) makes PoW cheap to bypass; adds latency for legitimate users for negligible attacker cost. |
| Email / SMS verification | Threat model says attacker has real accounts — pure friction, no benefit. |
| Datacenter ASN blocklists | False-positive risk on free-tier infra; bypassed by residential proxies ($0.16–4/GB in 2026); maintenance burden. |
| TLS / JA4 fingerprinting | Only OSS nginx module is officially "use at your own risk"; not viable on free-tier infra. |
| Per-user content watermarking | Useful for post-leak forensics, but doesn't reduce capture score. Mention in README, don't build. |
| Iocaine-style content poisoning (fake exercises) | `content_hash` is verified by the evaluator — can't serve fake content. Replaying real seen content achieves the same effect safely. |
| Hashed UUID storage | Best practice in prod, but adds zero anti-scraping value within the threat model. Mention in README. |
| Honeypot exercise IDs | We don't expose any ID-based endpoint, so there's no surface to trip. Would need a different attack vector. |

## 5a. Why linear progression isn't a product limitation

This is the hardest interview question we expect ("you optimized for security at the cost of product — real Duolingo lets users skip around"). The honest answer:

**Linear progression is the standard pattern in curriculum-driven EdTech.** Duolingo, Khan Academy, Brilliant, Babbel, Coursera, edX — all gate lessons by prerequisite completion. Lesson 7 assumes vocabulary from lesson 6; random access defeats the pedagogy.

The real flexibility in apps like Duolingo maps cleanly onto our design:

| Real-world UX feature | How it fits our design |
|---|---|
| Placement test → start at lesson N | Initialize `cursor = placement_result` on register. No architectural change. |
| Daily practice / review mode | Serves content already seen — same code path as our shadow-throttle (random from `[1, cursor]`). |
| Multiple skill tracks (speaking / listening / grammar) | Multiple cursors per user, one per track. Each track is linear; collapse property holds within each. |
| Section/unit checkpoint exam | Cursor jumps to next unit boundary on passing. Server still chooses what to serve. |
| Featured "lesson of the day" | Server selects from `[1, cursor]`. Same shadow-throttle code. |

None of these break the no-ID-lookup invariant or the parallel-account-collapse property. Adding them is a UX-surface change, not an architectural one.

**What this design genuinely can't model:** random-read products like full-corpus search, dictionary lookup, or per-user ML-driven personalization that requires arbitrary cross-corpus access. Those products need a different threat model and different defenses (DRM, per-payload encryption, or accepting they're scrapable).

## 6. Project layout

Mirrors the reference at `/Users/ivanlutsenko/kek/nai`: flat packages at module root, single `main.go`, Fiber + recover middleware, Viper for config, testify for tests, plain `database/sql`.

```
course-api/
  main.go
  go.mod
  README.md
  Dockerfile
  config/
    env_vars.go
    setup_env.go
  entities/
    exercise.go
    user.go
  server/
    add_routes.go
    middleware.go
    handlers_health.go
    handlers_register.go
    handlers_next.go
    handlers_complete.go
  services/
    course_service.go
    bot_detector.go
  db/
    db.go
    migrate.go
    migrations/
      0001_init.sql
  cmd/
    migrate/
      main.go
  scripts/
    migrate.sh
    register.sh
    next.sh
    complete.sh
    happy_path.sh
    test.sh
  tests/
    integration_test.go
  data-generator/         # from the brief, untouched
    main.go
```

Module name: `course-api`. Two binaries: the server (`main.go`) and the migrator (`cmd/migrate`).

## 7. Database

### Driver

`github.com/tursodatabase/libsql-client-go/libsql`. Registered as `sql.Open("libsql", url)`. Same driver everywhere:
- Local: `DB_URL=file:./course.db`
- Prod (Turso): `DB_URL=libsql://<host>?authToken=...`

### Schema (`db/migrations/0001_init.sql`)

```sql
CREATE TABLE schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at INTEGER NOT NULL
);

CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- keys: dataset_scale, dataset_seed, total_exercises

CREATE TABLE exercises (
    exercise_id  INTEGER PRIMARY KEY,    -- 1..N, == cursor value
    canonical_id TEXT    NOT NULL,        -- "ex_0000000001" from generator
    unit_id      INTEGER NOT NULL,
    lesson_id    INTEGER NOT NULL,
    content_gz   BLOB    NOT NULL         -- gzipped raw JSON record (preserves content_hash)
);
CREATE INDEX idx_exercises_unit ON exercises(unit_id);

CREATE TABLE user_progress (
    user_id         TEXT    PRIMARY KEY,  -- raw UUID v4 string
    status          TEXT    NOT NULL DEFAULT 'real',  -- 'real' | 'bot'
    cursor          INTEGER NOT NULL DEFAULT 0,
    is_active       INTEGER NOT NULL DEFAULT 0,
    active_exercise INTEGER,
    created_at      INTEGER NOT NULL
);

CREATE TABLE user_completion (
    user_id     TEXT    NOT NULL,
    slot        INTEGER NOT NULL,         -- exercise_id % 100
    exercise_id INTEGER NOT NULL,
    start_time  INTEGER NOT NULL,         -- unix millis
    end_time    INTEGER,                  -- unix millis, NULL until /complete
    PRIMARY KEY (user_id, slot)
);
CREATE INDEX idx_completion_user ON user_completion(user_id);

CREATE TABLE user_daily (
    user_id   TEXT    NOT NULL,
    day       TEXT    NOT NULL,           -- "YYYY-MM-DD" UTC
    completed INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, day)
);
```

Cursor starts at **0**; the first `/next` advances to 1 and serves `exercise_id=1`. Ring buffer keeps at most 100 completions per user.

## 8. Endpoints

### `GET /health`
Returns `{"ok": true, "scale": <float>, "seed": <int>}`. Used for Fly/Render healthcheck and evaluator smoke test. No auth.

### `POST /register`
1. Generate UUID v4 via `github.com/google/uuid` (uses `crypto/rand`).
2. `INSERT INTO user_progress (user_id, created_at) VALUES (?, ?)`.
3. Return `{"access_token": "<uuid>"}`.

No per-IP rate limit in v1 — see §5.

### Transaction isolation policy

Both `/next` and `/complete` open `BEGIN IMMEDIATE` to acquire the writer lock at transaction start (libSQL/SQLite). On `SQLITE_BUSY` / equivalent, retry up to **3 times with 10ms exponential backoff** (10/20/40ms). After exhausting retries, return `503`. This avoids the deferred→upgrade race that would otherwise produce `SQLITE_BUSY` under contention.

### `POST /next` (auth: `Authorization: Bearer <uuid>`)

```
BEGIN IMMEDIATE

1. row = SELECT status, cursor, is_active, active_exercise
         FROM user_progress WHERE user_id = ?
   if not found: ROLLBACK, return 401

2. # Idempotent retry path
   if row.is_active:
       ex = SELECT * FROM exercises WHERE exercise_id = row.active_exercise
       COMMIT, return decode(ex)

3. # Shadow throttle: flagged user, no state mutation
   if row.status == 'bot':
       rand_id = randint(1, max(row.cursor, 1))
       ex = SELECT * FROM exercises WHERE exercise_id = rand_id
       COMMIT, return decode(ex)

4. # Course-complete guard (H10)
   new_cursor = row.cursor + 1
   total = meta.total_exercises (cached at boot)
   if new_cursor > total:
       COMMIT, return 204 No Content with {"course_complete": true}

5. # Atomic advance with WHERE is_active=0 guard
   UPDATE user_progress
   SET cursor = cursor + 1,
       is_active = 1,
       active_exercise = cursor + 1
   WHERE user_id = ? AND is_active = 0
   # NOTE: SQL evaluates RHS against OLD row values; both `cursor + 1`
   # references resolve to the same new value.
   if affected_rows == 0:
       # raced — another /next won; re-read and return active
       ROLLBACK, retry whole handler from step 1

6. INSERT OR REPLACE INTO user_completion
       (user_id, slot, exercise_id, start_time, end_time)
   VALUES (?, new_cursor % 100, new_cursor, now_ms, NULL)

7. ex = SELECT * FROM exercises WHERE exercise_id = new_cursor
   COMMIT, return decode(ex)
```

### `POST /complete` (auth: `Authorization: Bearer <uuid>`)

Idempotent: a retry after a successful first call returns `200 OK` with the same shape, without double-counting daily quota or re-running dwell checks.

```
BEGIN IMMEDIATE

1. # Atomic flip of is_active. Only the FIRST caller succeeds; retries see 0.
   UPDATE user_progress
   SET is_active = 0
   WHERE user_id = ? AND is_active = 1
   RETURNING cursor, active_exercise, status   # the values BEFORE the update
   if no row (user_id missing):
       ROLLBACK, return 401
   if affected_rows == 0:
       # Either (a) no active exercise was set, or (b) this is a retry of an
       # already-completed call. Both should return 200 OK idempotently.
       COMMIT, return 200 {"ok": true, "idempotent": true}

2. # First successful call — record end_time
   now = now_ms()
   active_slot = active_exercise % 100
   UPDATE user_completion
   SET end_time = ?
   WHERE user_id = ? AND slot = active_slot

3. single_ms = SELECT (end_time - start_time)
                FROM user_completion
                WHERE user_id = ? AND slot = active_slot

4. avg_ms = SELECT AVG(d) FROM (
       SELECT (end_time - start_time) AS d
       FROM user_completion
       WHERE user_id = ? AND end_time IS NOT NULL
       ORDER BY start_time DESC
       LIMIT ?                        -- DWELL_WINDOW
   )

5. new_status = status
   if status == 'real':
       if single_ms < MIN_SINGLE_DWELL_MS:
           new_status = 'bot'
       elif avg_ms is not null and avg_ms < MIN_AVG_DWELL_MS:
           new_status = 'bot'

6. today = utc_date(now)
   INSERT INTO user_daily VALUES (?, ?, 1)
       ON CONFLICT (user_id, day) DO UPDATE SET completed = completed + 1
       RETURNING completed
   if completed > MAX_DAILY_LESSONS:
       new_status = 'bot'

7. if new_status != status:
       UPDATE user_progress SET status = ? WHERE user_id = ?

8. UPDATE user_progress
   SET active_exercise = NULL
   WHERE user_id = ?
   # is_active was already set to 0 in step 1

COMMIT, return 200 {"ok": true}
```

The exercise content is returned to a flagged user on the **next** `/next` call (random replay), not from `/complete`.

## 9. Config (env vars via Viper)

| Name | Default | Purpose |
|---|---|---|
| `PORT` | `3000` | Fiber listen port |
| `DB_URL` | `file:./course.db` | libsql connection string |
| `MIN_AVG_DWELL_MS` | `5000` | Avg dwell below this in rolling window → bot |
| `MIN_SINGLE_DWELL_MS` | `1500` | Any single dwell below this → bot |
| `DWELL_WINDOW` | `10` | Number of recent completions in the rolling avg |
| `MAX_DAILY_LESSONS` | `100` | Above this daily count → bot |
| `DATASET_SCALE` | `0.01` | Reported in `/health` and README |
| `DATASET_SEED` | `42` | Same |

Defaults chosen so v1 is sane out of the box; all tunable via env in the deploy.

## 10. Migration tool (`cmd/migrate`)

Run manually via `scripts/migrate.sh`:
```bash
go run ./cmd/migrate \
    --db=file:./course.db \
    --scale=0.01 \
    --seed=42 \
    --generator-dir=./data-generator
```

Behavior:
1. Open DB, ensure `schema_version` table exists.
2. Apply any missing migrations from `db/migrations/*.sql` in order (idempotent).
3. Read `meta.dataset_scale` and `meta.dataset_seed`.
4. If different from input (different scale OR different seed), regenerate:
   - **Wipe user state first** (different dataset → existing cursors point at wrong/missing exercises):
     - `DELETE FROM user_progress`
     - `DELETE FROM user_completion`
     - `DELETE FROM user_daily`
     - `DELETE FROM exercises`
     - Log a clear warning: `"DATASET CHANGE: wiping user state (was scale=X seed=Y, now scale=A seed=B)"`
   - Shell out to `go run <generator-dir> --scale=<scale> --seed=<seed>` → writes `data/units/*.ndjson` + `data/manifest.json`.
   - Stream each ndjson line, parse `Exercise`, gzip the raw JSON line, `INSERT` into `exercises`.
   - Write `meta.dataset_scale`, `meta.dataset_seed`, `meta.total_exercises`.
5. Idempotent: re-running with the same `--scale`/`--seed` is a no-op (skips step 4 entirely).
6. Confirmation prompt (`--yes` to skip) before any user-state wipe.

## 11. Middleware

- `recover.New()` — same as the reference, panic-safe.
- `auth` middleware in `server/middleware.go` for `/next` and `/complete` only:
  - parse `Authorization: Bearer <uuid>`
  - validate UUID v4 format
  - SELECT to confirm user exists (rate-limited by DB anyway; if needed later, add an in-memory LRU)
  - put `user_id` into `c.Locals("user_id")`

Deliberately skipped: logger middleware (free-tier CPU), CORS (single-origin), response compression (exercise content is already stored gzipped — handler streams the BLOB and sets `Content-Encoding: gzip`).

## 12. Capture math (goes prominently in README)

`--scale=0.01` ⇒ 25,000 total exercises. The brief's stated evaluation is **1 hour wall-clock, 1 machine, no horizontal scaling, no pre-existing warmed accounts**.

### Headline number for the brief's stated eval

| Attack within the brief's 1hr / 1 machine rules | Unique exercises captured | % of corpus |
|---|---|---|
| **1 fresh account, dwell-compliant pacing (≥5s avg, ≥1.5s min)** | **≤ 100 (daily cap)** | **≤ 0.4%** |
| N fresh accounts in parallel within the same hour | ≤ 100 (all collapse to identical sequence 1..100 — see §3) | **≤ 0.4%** |
| 1 account sprinting under the dwell floor | ~2–3 lessons before flag, then random shadow replay (no new uniques) | **~0.01%** |

**Realistic worst case under the brief's eval: 0.4%.**

### Attacks outside the brief's stated eval (documented for completeness)

These require capabilities the brief explicitly excludes (multi-day budget, horizontal scaling) but are listed so the discussion in §15 has context.

| Out-of-scope attack | Capture % |
|---|---|
| 100 accounts pre-warmed to distinct cursors over 5+ days at the dwell-time floor, then a 1hr scrape on eval day | ~40% (100 accounts × 100/day cap) |
| Same as above + automated pre-warm detection enabled (§15) | < 5% (most accounts caught during warm-up) |

The pre-warm attack is the only meaningful weakness and is mitigated by the pre-warm detection described in §15 (not built in v1). It is **not** part of the brief's stated evaluation.

## 13. Why raw UUID, not hashed

For an interview test scored on capture % within a 1-hour window:
- Hashing UUIDs protects against credential-DB-leak replay attacks. That's not in the threat model.
- Raw UUID storage with a UUID-format check is simpler, faster, and has zero anti-scraping cost.
- In a real prod system this would be hashed (or a JWT, or rotated session tokens). Documented in §15.

## 13a. Auth model: "one UUID = one user"

The brief lets us define what constitutes "one user" for evaluation. Our definition:

> **One successful `/register` response = one UUID = one user.** The evaluator's "minimum number of valid client sessions per user" is therefore exactly **one UUID per learner**. Two UUIDs created from different `/register` calls are two distinct users, even if the same physical person operates them.

Why this is honest and defensible:

1. **The collapse property is the actual defense, not auth strength.** Two UUIDs from the same person follow identical content sequences (both start at cursor=0, both advance linearly). The system does not benefit from conflating them, nor does the attacker benefit from operating multiple UUIDs.
2. **A real product needs this same property.** Family-shared devices, work/personal accounts, anonymous trial → user-with-account: all create multiple identifiers per physical person. Treating each as a fresh user is correct UX behavior.
3. **The brief does not require identity verification.** It explicitly says the attacker has real paid accounts; SMS/email is excluded from the threat model.

What this is NOT:
- Not a defense against bulk account creation (no defense needed: see §3 collapse property).
- Not a claim that UUIDs prove human identity (they don't, and we don't need them to).

## 14. Tests (`tests/integration_test.go`)

Setup helper spins up an in-process Fiber app pointing at a temp libsql `file:` DB with a mini fixture dataset (~20 exercises).

1. `TestHappyPath` — register → next → wait → complete → next returns different exercise.
2. `TestNextIdempotent` — register → next → next (same exercise returned, no double-advance).
3. `TestCompleteIdempotent` — register → next → complete → complete (returns 200 with `idempotent: true`, daily count incremented only once).
4. `TestBotFlagBySingleDwell` — register → next → complete after 200 ms → status=bot → next returns random ≤ cursor, cursor doesn't advance.
5. `TestBotFlagByDailyCap` — env override `MAX_DAILY_LESSONS=3` → do 4 paced cycles → 4th flips status to bot.
6. `TestRingBufferOverwrite` — do 105 paced cycles → assert `user_completion` has exactly 100 rows for the user.
7. `TestUnauthorized` — `/next` and `/complete` without auth → 401.
8. `TestContentHashRoundTrip` — register → /next → gunzip body → parse JSON → assert payload's `content_hash` field equals the hash the generator wrote for that `exercise_id`. Guarantees the storage path preserves the field bit-for-bit.
9. `TestCourseComplete` — load a 3-exercise fixture; complete all 3; 4th `/next` returns `204 No Content` with `{"course_complete": true}`; cursor stays at 3.

## 15. What I'd do with two more weeks

- **Pre-warm detection**: flag accounts with dwell variance < 0.5 s or active >12 hr/day during the days before the eval. Defeats the only realistic remaining attack.
- **Per-user content watermarking**: deterministic distractor reordering per `(user_id, exercise_id)` for post-leak forensics.
- **Hashed UUID storage** with a thin per-process LRU cache for the auth path.
- **Per-IP register rate limit** as DoS hygiene (Fiber `limiter` middleware).
- **ASN-aware suspicion scoring**: free MaxMind GeoLite2-ASN, datacenter ASNs get stricter dwell/daily thresholds. Not a hard block.
- **Honeypot fields** (e.g. an unused `/exercise/{id}` route that 404s normally; any hit = flag) — surface for a second-pass defense.
- **Content-level audit log** for forensics: which user saw which `exercise_id` at what time. Currently we only keep the last 100 per user.
- **Smarter shadow content**: serve previously-completed exercises in plausible "review" order rather than uniform random, so the attacker can't detect they've been flagged by anomalously distributed shadow IDs.
- **Move data tier to object storage** for scale=1.0 / 10 GB / 100 GB. The access pattern (server picks exercise by cursor → fetch one row) is unchanged because no ID-lookup endpoint exists. SQLite holds the index, content lives in R2/S3.

## 16. What's explicitly out of scope

- **Rooted-device memory capture.** Once the bytes are on a compromised device, they're gone. We're driving aggregate cost, not single-device confidentiality.
- **Hiring 10,000 humans to manually play through the course.** No technical defense exists for this; price-of-content beats us if labor is cheap enough.
- **Email/SMS verification.** Threat model has real paid accounts; verification adds friction with no security benefit in this model.
- **Full anti-cheating instrumentation.** Out of scope; we care about exfiltration, not whether the learner is actually learning.

## 17. Build order (with verification gate at each step)

Each step has an explicit pass condition. Do not advance until all pass.

| # | Step | Pass condition |
|---|---|---|
| 1 | `go.mod`, `main.go`, `config/`, `entities/` skeletons | `go build ./...` succeeds; `go vet ./...` clean; binary starts and exits cleanly on SIGTERM |
| 2 | `db/db.go`, `db/migrate.go`, `db/migrations/0001_init.sql` | `go run ./cmd/migrate --db=file:./test.db --skip-data` creates all tables; re-run is a no-op; `schema_version` has row 1 |
| 3 | `cmd/migrate/main.go` data loader | Run with `--scale=0.01 --seed=42`; `SELECT COUNT(*) FROM exercises` = manifest's `total_exercises`; one row's `content_gz` gunzips and re-parses to a valid Exercise whose `exercise_id` matches `ex_<padded>` of its row |
| 4 | `services/course_service.go`, `services/bot_detector.go` | Compiles; private unit tests on `bot_detector` cover the single-dwell, rolling-avg, daily-cap branches |
| 5 | `server/middleware.go`, `server/handlers_health.go` | `curl /health` returns `{"ok":true,...}` with declared scale/seed |
| 6 | `server/handlers_register.go` | `curl -X POST /register` returns a UUID v4; row exists in `user_progress` with cursor=0, status='real' |
| 7 | `server/handlers_next.go` | After `/register`, `/next` returns exercise with `canonical_id == "ex_0000000001"` after gunzip; second `/next` (no `/complete`) returns the same exercise; `cursor` and `is_active` are 1 |
| 8 | `server/handlers_complete.go` | After `/complete` with 5s wait, `/next` returns `ex_0000000002`; doing two cycles in <500ms total flips status to 'bot'; subsequent `/next` returns a random exercise with id ≤ cursor and does **not** advance cursor. `/complete` is idempotent: calling twice in a row counts daily quota once and returns 200 both times. |
| 9 | `tests/integration_test.go` | All 9 tests in §14 green; `go test ./...` passes; no flakes on 3 reruns |
| 10 | `scripts/*.sh` + `Dockerfile` | `./scripts/happy_path.sh` against `localhost:3000` end-to-end green; `docker build .` succeeds |
| 11 | `README.md` | Capture math table present; threat model present; defenses with rationale; rejected defenses with rationale; two-weeks list; out-of-scope list; deployed URL placeholder + `--seed`/`--scale` declared |

Global invariants (re-check after every step):
- `go build ./...` succeeds.
- `go vet ./...` clean.
- All prior step tests still pass (`go test ./...`).
- Legitimate happy path `register → next → complete → next` returns sequentially correct exercises.
