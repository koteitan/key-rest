[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# TLA+ Formal Specification

This directory contains a TLA+ specification and TLC model-checking configurations
for the key-rest credential masking pipeline.

## What is verified

`MaskingPipeline.tla` is a single parameterized spec driven by four constants.
Different `.cfg` files vary those constants to verify different properties.

### Encoding coverage (non-vacuous)

`caught` is derived from step coverage sets (`Covered`), **not** by listing
`ServerEncodings`.  `ServerEncodings` is the adversary's choice set; `Covered`
is what the pipeline actually handles.  Any encoding outside `Covered` causes
TLC to find a `NoCredentialLeak` violation.

| Encoding | Caught by | Lean analogue |
|---|---|---|
| `none` | — (credential absent) | — |
| `raw` | Step 2: `maskCredentials` | `masks_raw` |
| `json_esc` | Step 2: `maskCredentials` (JSON form) | `masks_json_esc` |
| `b64` | Step 1: `maskTransformOutputs` | `masks_b64` |
| `pct_raw` | Step 4: tolerant-decode → Step 2 | `masks_pct_raw` |
| `pct_b64` | Step 4: tolerant-decode → Step 1 | `masks_pct_b64` |
| `pct_stray` | Step 4: tolerant-decode (v1.0.1 fix) | `fixed_masks_stray_pct` |
| `pct_plus_b64` | Step 4: tolerant-decode, `+` kept (v1.0.2 fix) | `fixed_masks_plus_b64` |
| `trunc` | Step 3: `maskTruncatedKeys` (known API only) | — |

### Configuration files

| Config | Constants | Expected result |
|---|---|---|
| `MaskingPipeline.cfg` | All covered encodings, fixed decoder, snapshot on, step 4 on | **No error** |
| `MaskingPipeline_negative.cfg` | Adds `base64url`, `hex`, `rot13` to `ServerEncodings` | **VIOLATED** — non-vacuity proof |
| `MaskingPipeline_buggy.cfg` | `ExtraStep4Catches={}` (pre-v1.0.2 decoder) | **VIOLATED** — `pct_stray` / `pct_plus_b64` slip through |
| `MaskingPipeline_naive.cfg` | `UseSnapshot=FALSE` (no snapshot TOCTOU guard) | **VIOLATED** — `DisableKey` race causes leak |
| `MaskingPipeline_disorder.cfg` | `Step4Enabled=FALSE` (step 4 omitted) | **VIOLATED** — `pct_raw` / `pct_b64` slip through |

### Universality

TLC exhaustively explores **all** combinations of:
- `serverEncoding` ∈ `ServerEncodings` (adversary's nondeterminism)
- `apiKnown` ∈ {`TRUE`, `FALSE`} (known / unknown API prefix)
- `placement` ∈ {`"header"`, `"body"`} (agent's placement choice)
- `DisableKey` interleaved with any phase from `snap` through `s4` (all concurrent disable/reload races)

### Invariants

| Invariant | Description |
|---|---|
| `TypeOK` | Type correctness |
| `NoCredentialLeak` | Agent never sees raw credential (except documented `IntentionalLimit`) |
| `ResolvedImpliesAllowed` | Credential forwarded only when placement was allowed |

## Prerequisites

- Java runtime (JRE 11+)
- [TLA+ tools](https://github.com/tlaplus/tlaplus/releases) — `tla2tools.jar`

Place `tla2tools.jar` in the repository root (two levels above this directory).

## Verification

```sh
# from repo root
java -jar tla2tools.jar -config formal/tla/MaskingPipeline.cfg formal/tla/MaskingPipeline.tla
```

Run all configs:

```sh
for cfg in formal/tla/MaskingPipeline*.cfg; do
    echo "=== $cfg ==="
    java -jar tla2tools.jar -config "$cfg" formal/tla/MaskingPipeline.tla 2>&1 | tail -3
done
```

Expected: the positive cfg ends with `No error has been found`; all four
negative cfgs end with an `Invariant NoCredentialLeak is violated` error.
