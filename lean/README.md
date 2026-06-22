[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# Lean Formal Proofs

This directory contains kernel-verified Lean 4 proofs for the key-rest
credential masking pipeline.

## What is verified

`MaskingPipeline.lean` proves that the fixed pipeline (commit 45382b3) safely
masks credentials for all five server encoding scenarios:

| Theorem | Scenario |
|---|---|
| `masks_raw` | Credential appears verbatim in the response |
| `masks_json_esc` | Credential appears JSON-escaped (e.g. newline → `\n` literal) |
| `masks_b64` | Base64 transform output appears verbatim |
| `masks_pct_raw` | Percent-encoded raw credential (e.g. `ab%7Ede`) |
| `masks_pct_b64` | Percent-encoded base64 output (e.g. `YWJ%2BZGU%3D`) |

All theorems are proved with the `decide` tactic, which means the Lean kernel
itself reduces the computation and confirms the result — no manual proof steps
or axioms beyond the kernel are involved.

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

No output means all theorems are verified. Any error is printed to stderr.

Alternatively, use Lake (the Lean build system):

```sh
cd lean
lake build
```
