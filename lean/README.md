[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Lean Formal Proofs

This directory contains kernel-verified Lean 4 proofs for the key-rest
credential masking pipeline.

`MaskingPipeline.lean` has two complementary layers.

## Part 1 — regression anchors (`decide`)

Closed propositions that pin specific server-encoding scenarios for the fixed
test vectors `cred = "ab~de"`, `b64cred = "YWJ+ZGU="`, `uri = "u/k"`. These are
kernel-verified test vectors, not general theorems — but they are cheap and
exact, and they lock down the concrete bypasses.

| Theorem | Scenario |
|---|---|
| `masks_raw` | Credential appears verbatim in the response |
| `masks_json_esc` | Credential appears JSON-escaped (e.g. newline → `\n` literal) |
| `masks_b64` | Base64 transform output appears verbatim |
| `masks_pct_raw` | Percent-encoded raw credential (e.g. `ab%7Ede`) |
| `masks_pct_b64` | Percent-encoded base64 output (`YWJ%2BZGU%3D`) — the bypass fixed in 45382b3 |

## Part 2 — universal theorems (induction)

`∀`-quantified statements that promote the point checks to general guarantees
where that is sound.

| Theorem | Statement |
|---|---|
| `percent_roundtrip` | `percentDecodeAll (percentEncodeAll s) = s` for every low-byte string `s` |
| `infix_replaceAll_false` | `replaceAll` removes every occurrence of a nonempty needle that is char-disjoint from a nonempty replacement |
| `masks_cred_universal` | For **any** `uri`, `b64cred`, `response`, a nonempty credential char-disjoint from its template never appears verbatim in the pipeline output |

`masks_cred_universal` is strictly stronger than `masks_raw`: it quantifies over
all four inputs, not one fixed vector. The proof works because `maskCredentials`
(`replaceAll cred (key-rest://uri) …`) is the **outermost** operation in both
branches of `maskPercentEncoded`, so the whole pipeline inherits the absorption
lemma.

Every theorem is proved without `sorry`/`admit`/extra axioms. Each ends with a
`#print axioms` check; the only dependencies are `propext`, `Classical.choice`,
`Quot.sound` (the standard Lean kernel axioms).

## Part 2c — faithful decode model (now detects the bug)

An earlier version of this model decoded percent-encoding with a tolerant,
total function and omitted the implementation's "decode failed → return the
body unmasked" branch. That gap is exactly where a real credential-leak lived,
and the proof could not see it. The model is now faithful to the decoder's
failure behaviour:

| Definition / theorem | What it captures |
|---|---|
| `queryUnescape` | Go `url.QueryUnescape` modeled honestly: all-or-nothing (a single bad `%` → `none`) and `'+'`→space |
| `pipeline_buggy` | The old code path that bails unmasked on decode failure |
| `buggy_leaks_stray_pct` | **Proves the leak**: a stray invalid `%` in the body leaves a percent-encoded credential recoverable (`= true`) |
| `fixed_masks_stray_pct` | The fixed `pipeline` masks the same input (`= false`) |
| `fixed_masks_plus_b64` | v1.0.2 fix verified: always using `percentDecodeAll` keeps `'+'` as `'+'`, so the `'+'`-containing base64 form is now masked (`= false`) |

Lesson: a formal proof only guarantees properties **of the model**. Bugs in the
gap between model and implementation — here, error handling abstracted into a
happy-path total function — are invisible until the model is made faithful to
that path. (Ironically, the old tolerant decoder *was* the fix.)

### Model fidelity — still simplified

- `maskCredentials` models **one** credential; the implementation sorts **many**
  longest-first (which is why two overlapping keys can partially mask each other).
- `maskTruncatedKeys` (regex, OpenAI/Stripe/localhost only) is omitted; it is an
  extra masker, so omitting it is conservative for the leak proofs.
- Only the response **body** is modeled; the implementation runs the same
  sequence on response **headers**, so the guarantees transfer.

## A finding surfaced by universalization

`percent_roundtrip` is **not** unconditional. The Go test-server percent-encodes
**bytes**, while this model percent-encodes **codepoints** (`Char.toNat`). For a
char with `toNat ≥ 256` the nibble helper overflows the `0..15` hex range
(`Char.ofNat 256` encodes to `"%G0"`, which does not decode back), so the round
trip only holds under a low-byte side condition (every char `< 256`). This holds
for all ASCII/Latin-1 credentials (real API keys); it is a model/Go divergence
worth recording, not a proxy bug.

## What is NOT proved (scope)

These proofs cover the **masking layer only** (response scanning). key-rest's
primary defence is **placement validation** (default-deny on where a credential
may be sent); masking is a secondary block-list net. So even a fully universal
masking proof would not mean "credentials never leak".

The following are documented in the source as limits, not proved, because they
are genuinely false or out of reach:

- **`∀ cred` is false.** A credential equal to a substring of the replacement
  template (`key`, `rest`, `://`, uri text) survives masking. The `Disj`
  side condition rules these out by construction.
- **`∀ encoding` is false by mechanism.** Masking enumerates raw / json-esc /
  base64 / percent / percent+base64 / truncated. Anything outside the list slips
  through: base64url, hex, ROT-N, **double** percent-encoding (the proxy decodes
  only one layer), inter-character spacing, one-char-at-a-time spelling, stream
  splitting. A block-list is inherently incomplete.
- **Free-form LLM response text cannot be contained** — full alphabet, unbounded
  length; neither allow-list nor block-list constrains it.

## A design decision baked into `infix_replaceAll_false`

The single-pass `replaceAll` does **not** rescan the seam between an inserted
replacement and the following text, so
`∀ cred body. isInfix cred (replaceAll cred repl body) = false` is **false** in
general (e.g. `replaceAll "ab" "a" "abb" = "ab"`). The universal theorem
therefore carries a `Disj` (char-disjoint) side condition — sufficient, though
stronger than the truly necessary "no suffix of repl is a prefix of cred". This
is why `masks_cred_universal` covers a disjoint-charset credential while the
realistic `"ab~de"` (which shares `'e'` with `key-rest://`) stays a Part 1
pointwise anchor. Whether to keep the current `replaceAll` + side condition or
switch to a rescanning `replaceAll` (provable from `cred ∉ repl` alone, at the
cost of a harder termination/coverage proof) is an open design choice.

## Prerequisites

- [Lean 4](https://lean-lang.org/) — version specified in `lean-toolchain` (4.30.0)

Install via [elan](https://github.com/leanprover/elan):

```sh
curl https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh -sSf | sh
```

## Verification

```sh
cd lean
lean MaskingPipeline.lean
```

The `#print axioms` lines print the (standard) axiom dependencies; any real error
is printed to stderr. A clean run with only those lines means all theorems are
verified.

Alternatively, use Lake (the Lean build system):

```sh
cd lean
lake build
```
