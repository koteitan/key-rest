[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Formal Verification

This directory contains two complementary formal verification efforts for the
key-rest credential masking pipeline.

## Links

| Tool | Directory | Purpose |
|---|---|---|
| Lean 4 | [formal/lean](lean/README.md) | Kernel-verified proofs of string masking correctness |
| TLA+ / TLC | [formal/tla](tla/README.md) | Model-checked pipeline orchestration, TOCTOU, placement gate |

## Scope of Verification

### What Lean covers

Lean proves properties of the **masking functions themselves** — the string
operations that search and replace credentials in a response body.

| Layer | What is proved |
|---|---|
| Regression anchors (Part 1) | Five specific encoding scenarios are correctly masked for fixed test vectors (`decide`, kernel-verified) |
| Universal theorems (Part 2) | `percent_roundtrip`, `infix_replaceAll_false`, `masks_cred_universal` — hold for **all** inputs satisfying side conditions |
| Bug detection (Part 2c) | `buggy_leaks_stray_pct` proves the pre-v1.0.1 bug existed; `fixed_masks_stray_pct` and `fixed_masks_plus_b64` prove both fixes are correct |

Lean cannot verify: concurrency, step ordering, placement gate, or
properties that depend on program state over time.

### What TLA+ covers

TLA+ verifies **pipeline orchestration** — how the steps are composed,
ordered, and protected against concurrent modification.

| Property | Verified by |
|---|---|
| All covered encodings are masked | `MaskingPipeline.cfg` — `NoCredentialLeak` holds |
| Uncovered encodings are not masked (non-vacuity) | `MaskingPipeline_negative.cfg` — `NoCredentialLeak` violated for `base64url`, `hex`, `rot13` |
| Pre-v1.0.2 buggy decoder leaks | `MaskingPipeline_buggy.cfg` — violated for `pct_stray`, `pct_plus_b64` |
| Snapshot prevents concurrent-disable TOCTOU | `MaskingPipeline_naive.cfg` — violated without snapshot |
| Step 4 is load-bearing (order matters) | `MaskingPipeline_disorder.cfg` — violated when step 4 is omitted |
| Placement gate blocks bad-placement requests | `ResolvedImpliesAllowed` — holds in all positive cfgs |

TLA+ cannot verify: string-level masking correctness (infinite string domain),
or properties of the Go runtime and library functions.

### What neither tool covers

- **Primary defence is placement validation** (default-deny on where a credential
  may be sent). Masking is a secondary block-list net. Even a fully universal
  masking proof would not mean "credentials never leak".
- **`∀ encoding` is false by mechanism.** Masking enumerates a fixed set of
  encoding forms. Anything outside the list (base64url, hex, double
  percent-encoding, inter-character spacing, stream splitting, …) bypasses it.
  A block-list is inherently incomplete.
- **Free-form LLM response text** — full alphabet, unbounded length. Neither
  allow-list nor block-list constrains it.

## Universality

| Dimension | Lean | TLA+ |
|---|---|---|
| Response body content | **∀ string** (all strings, inductive) | Finite set of encoding symbols |
| Credential value | **∀ string** (with `Disj` side condition) | Single abstract credential |
| URI / template | **∀ string** | Fixed abstract model |
| Encoding form | 8 concrete scenarios | **∀ enc ∈ ServerEncodings** (all explored) |
| `apiKnown` | Not modeled | **∀ {TRUE, FALSE}** (both explored) |
| Placement | Not modeled | **∀ {"header", "body"}** (both explored) |
| Concurrent disable/reload | Not modeled | **∀ interleavings** (all phase combinations) |
| Decoder correctness | Proved for `queryUnescape` / `percentDecodeAll` | Abstracted (encoding symbols, not strings) |

The two tools are **complementary**: Lean quantifies over infinite string
domains where TLC cannot reach; TLA+ quantifies over concurrent interleavings
and program states where Lean's inductive proofs do not apply.
