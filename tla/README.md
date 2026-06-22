[← Back](../README.md) | [English](README.md) | [Japanese](README-ja.md)

# TLA+ Formal Specification

This directory contains a TLA+ specification and TLC model-checking configuration
for the key-rest credential masking pipeline.

## What is verified

`MaskingPipeline.tla` models the masking pipeline as a state machine and checks
that `NoCredentialLeak` holds for all seven server encoding strategies:

| Encoding | Description | Caught by |
|---|---|---|
| `none` | Credential absent from response | — |
| `raw` | Raw credential bytes echoed verbatim | Step 2: `maskCredentials` |
| `json_esc` | JSON-escaped credential (e.g. `"` → `\"`) | Step 2: `maskCredentials` (JSON form) |
| `b64` | Base64 transform output verbatim | Step 1: `maskTransformOutputs` |
| `pct_raw` | Percent-encoded raw credential | Step 4: URL-decode → `maskCredentials` |
| `pct_b64` | Percent-encoded base64 output | Step 4: URL-decode → `maskTransformOutputs` |
| `trunc` | Truncated key pattern (e.g. `sk-****abcd`) | Step 3: `maskTruncatedKeys` |

## Prerequisites

- Java runtime (JRE 11+)
- [TLA+ tools](https://github.com/tlaplus/tlaplus/releases) — `tla2tools.jar`

Place `tla2tools.jar` in the repository root (one level above this directory).

## Verification

```sh
java -jar ../tla2tools.jar -config tla/MaskingPipeline.cfg tla/MaskingPipeline.tla
```

Expected output ends with:

```
Model checking completed. No error has been found.
```
