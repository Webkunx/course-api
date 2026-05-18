# data-generator

Generates a synthetic Spanish-language course corpus. Deterministic per
`--seed`. Parallel across units.

## Usage

```bash
go run . --out=../data --scale=1.0 --seed=42
```

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `--out` | `./data` | Output directory. Will create `units/` and `manifest.json` inside it. |
| `--scale` | `1.0` | Fraction of full corpus. `1.0` → 500 units / 2.5M exercises / ~10 GB. `0.01` → 5 units / ~100 MB. |
| `--seed` | `42` | Base RNG seed. Same `seed`+`scale` always produces byte-identical output. |
| `--concurrency` | `NumCPU` | Parallel unit workers. |

## Output layout

```
data/
├── manifest.json              # totals, seed, scale, structure
└── units/
    ├── u_001.ndjson           # one JSON exercise per line
    ├── u_002.ndjson
    └── ...                    # 500 files at scale 1.0
```

Each line of a unit file is a complete `Exercise` JSON object. See
`main.go` for the schema. Key fields:

- `exercise_id` — globally unique, format `ex_%010d`, contiguous from
  `ex_0000000001` to `ex_<total_exercises>`.
- `content_hash` — `base64url(sha256(exercise without content_hash))[:22]`.
  Used by `attack-measure` to verify captured records weren't fabricated or
  silently corrupted.

## Reference timings (laptop, NVMe SSD)

| Scale | Units | Exercises | Approx size | Approx time (8 cores) |
|---|---|---|---|---|
| 0.001 | 1 | 5,000 | ~20 MB | <1 s |
| 0.01 | 5 | 25,000 | ~100 MB | ~2 s |
| 0.1 | 50 | 250,000 | ~1 GB | ~20 s |
| 1.0 | 500 | 2,500,000 | ~10 GB | ~3-4 min |
