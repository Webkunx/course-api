# Original Task Brief — Anti-Scraping Course API

## Challenge: Serve 10 GB of course content without letting anyone steal it

### The scenario

You're the new backend lead at a Duolingo-style language-learning startup. The company has spent two years building a proprietary Spanish course: lessons, exercises, grammar explanations, cultural notes, audio scripts. The corpus is ~10 GB of JSON and it's the entire business.

We're shipping a mobile + web client. The client needs to fetch course content on demand so a learner can progress through lessons. The catalog is mostly static — content changes maybe weekly.

A competitor has hired contractors to clone our product. They will:

- create real user accounts (paid or trial),
- run the client in instrumented browsers / rooted phones,
- feed responses into LLM-driven scrapers that adapt to whatever you build,
- combine accounts and IPs to scale horizontally.

**Goal:** make full-corpus exfiltration economically and operationally infeasible — meaning that obtaining a meaningful fraction of the data either takes so long, costs so much, or requires so many accounts that doing it is not worth it relative to just paying licensing fees or building their own content.

You are **not** trying to make it impossible. A determined adversary with unlimited time will always extract some amount; your job is to drive cost per exfiltrated unit as high as you can while keeping the legitimate learner experience excellent.

### What you must deliver

1. **A deployed, reachable HTTPS URL** where your API is running.
   - Free-tier hosting only — Cloud Run, Render, Fly.io, Railway, Vercel, AWS Lambda + API Gateway, Deno Deploy, fly machines, anything similar. No personal credit card limits expected.
   - The URL must stay up for at least 7 days from the day you submit. If it sleeps on idle, make sure the first request wakes it within a reasonable bound.
   - You pick the deployment region. We will attack from a US/EU laptop.

2. **A link to a GitHub repository** containing the full source.
   - Public, or private with the interviewer added.
   - One commit at the end is fine; a clean history is nicer.

3. **The exact dataset configuration you deployed against** — `--seed=N` and `--scale=X`, declared in your README so the corpus can be regenerated locally to score the attack.

### Repository contents required

- The Go server source code. Module-y, idiomatic, your call on layout.
- A `README.md` with:
  - the deployed URL,
  - the `--seed` and `--scale` you generated the deployed dataset with,
  - a documented description of the API (endpoints, payloads, auth flow, how a legitimate client uses it end-to-end),
  - your threat model: who you're defending against and what their capabilities are,
  - your defenses: each one named, with one paragraph on what it does and why you chose it,
  - your deployment story: which platform, why, how you handle the data, what the free-tier limits forced you to compromise on,
  - what you'd do with another two weeks,
  - what you explicitly chose not to defend against, and why,
  - optional — a `make run` (or `docker compose up`) that starts the same server locally.
- A way for a legitimate client to actually use the API (curl script, small CLI, or examples).

You may use any libraries, any DB, any infra within free tiers.

### The dataset

Run the generator in `data-generator/`. It produces:
- `data/units/u_XXX.ndjson` — one file per unit, one JSON exercise per line
- `data/manifest.json` — total counts, seed, course structure

Exercise record shape is defined in `data-generator/main.go`. Key fields per record:
- `exercise_id` — globally unique, format `ex_%010d`, contiguous from `ex_0000000001` to `ex_<total_exercises>`.
- `content_hash` — used by evaluator to detect tampering of captured payloads. Do not strip or invent these on the server side.

Total dataset at `--scale=1.0`: ~10 GB across 2.5M exercises in 500 units.

Reasonable scale choices:
- `--scale=0.01` (~100 MB, 25K exercises)
- `--scale=0.1` (~1 GB, 250K exercises)
- `--scale=1.0` (~10 GB)

### How evaluation works

**1. Attack pass**

- Open deployed URL and confirm it responds.
- Regenerate dataset locally using declared `--seed` and `--scale`.
- Create the minimum number of valid client sessions that your auth model defines as "one user".
- Run a Claude-Code-driven scraping campaign for ~1 hour wall-clock budget, one machine, no cloud horizontal scaling.
- Collect every payload the attacker observes.
- Measure unique `exercise_id` values recovered as % of total corpus deployed. That's the **capture score** (lower is better).

Confirm the legitimate happy path: a normal learner moving through lessons must see correct content with **P95 latency < 400 ms** on a content fetch (excluding cold-start; warmed first).

**2. Architecture conversation (≈ 45 min)**

This is the bigger weight. Be ready to:
- Walk through threat model and defenses.
- Defend each trade-off, including the ones chosen against.
- Discuss where the attacker gets through first and what the second-pass defenses would be.
- Talk through "what changes if the dataset is 100 GB, or 1 TB" and "what changes at 10× user count".

### Hard rules

- **Go** for the server. Other tools/services around it are fine.
- Deployed on a free tier, reachable over public HTTPS. No IP-allow-listing the interviewer in.
- No "just don't serve the data." A legitimate client must be able to render any lesson the learner navigates to.
- No paid bot-protection services (no AWS WAF, Cloudflare Bot Management, Datadome, PerimeterX, etc.). Free Cloudflare proxy / basic Cloudflare rules in front of origin is allowed.
- Don't modify the generator's data shape. The canonical `exercise_id` field must be recoverable from whatever a legitimate client receives.

### Time

Expected: focused 4–8 hours. The write-up is weighted heavily.
