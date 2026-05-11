[English](testing.md) | [Japanese](testing-ja.md)

# Test Layout

key-rest tests fall into three categories: per-package unit tests, multi-language system tests, and PoC / regression tests for confirmed security findings.

## Per-package unit tests

Each Go package under `internal/` and `cmd/` has its own `*_test.go` files. They run with the standard `go test` workflow and have no external dependencies.

| Package | Location | Scope |
|---|---|---|
| `internal/crypto` | `crypto_test.go` | AES-256-GCM round-trip, PBKDF2 key derivation, mlock / zero-clear primitives |
| `internal/keystore` | `keystore_test.go`, `keystore_extra_test.go` | `Add` / `Remove` / `DecryptAll` / `Lookup` / `Disable` / `Enable` / `ListStatus`, on-disk format, file-permission checks, error paths |
| `internal/uri` | `uri_test.go`, `uri_bytes_test.go` | `FindAll`, `Replace`, `ReplaceBytes`, transform-function plumbing, `parseArgs` edge cases |
| `internal/proxy` | `proxy_test.go`, `coverage_test.go`, `headerinject_poc_test.go`, `raceattack_poc_test.go`, `raceattack2_poc_test.go`, `bodysmuggle_poc_test.go` | Request validation, URI resolution, response masking, secure-transport wire building, F-001 / F-002 / F-003 regressions |
| `internal/server` | `server_test.go`, `server_extra_test.go` | Unix-socket protocol dispatch (`http` / `reload` / `enable` / `disable` / `list` / `version`), connection limits, malformed-input handling |
| `internal/daemon` | `daemon_test.go` | `IsRunning`, `Start` / `Stop` lifecycle (with in-test SIGTERM trap), `reload` / `enable` handlers |
| `cmd/key-rest` | `main_pure_test.go`, `main_test.go` | Pure helpers like `formatPlacement`, plus subprocess tests of the built binary for `version` / `status` / unknown-command / `help` / `stop`-without-daemon |

Run with:

```bash
make test-go      # all packages, fail fast on regressions
make coverage     # text summary
make coverage-html # browsable HTML report at coverage.html
```

## System tests

Cross-language end-to-end tests live under `system-test/`. They start the real `key-rest` daemon, register all bundled service credentials, and drive every supported client (curl, Go, Python, Node.js) against the bundled `test-server`. See [`system-test/README.md`](../system-test/README.md) for details.

| Suite | Location | Driver |
|---|---|---|
| Curl | `system-test/curl/system-test.sh` | bash |
| Go | `system-test/go/system_test.go` | `go test` |
| Python | `system-test/python/system_test.py` | python3 |
| Node.js | `system-test/node/system_test.mjs` | node |

Run all suites with:

```bash
make test-system
```

Each suite expects the project binary to be built (`make build`) and the test-server certificate to be present.

## Security PoC / regression tests

Tests under `internal/proxy/` named `*_poc_test.go` started life as proofs of concept for the SPECA-style audit findings (see [`audit-speca.md`](audit-speca.md)). After each fix landed, the assertions were inverted so they now act as regressions:

- `headerinject_poc_test.go` — F-001 (header-key CRLF injection)
- `raceattack_poc_test.go` — F-002 (Disable / mask race), with both a deterministic gated variant and a 200-iteration natural-timing variant (the latter skipped under `-short`)
- `raceattack2_poc_test.go` — F-003 (reload-after-`keys.enc`-tamper race)
- `bodysmuggle_poc_test.go` — supplementary checks (Transfer-Encoding strip, validation race)

Run them under the race detector to verify there are no underlying data races:

```bash
go test -race -short ./internal/proxy
```

## CI

`.github/workflows/test.yml` runs the unit tests on every push to `main` and every pull request, prints the coverage summary, and uploads `coverage.out` as a build artifact.

## Test-server

`test-server/` provides a localhost HTTPS server that mocks every supported third-party service (OpenAI, Anthropic, Stripe, etc.). It is consumed by both system tests and (transitively, via the certificate) the hacking-challenge environment. The server itself has no `*_test.go` files — it is verified through the system-test suites that drive it.

## Test tree

Every `Test*` function across the project, grouped by file.

- `clients/go/client_test.go`
  - `TestClientDo`
  - `TestClientError`
  - `TestClientConnectionError`
  - `TestNewClient`
  - `TestClientPost`
- `cmd/key-rest/main_pure_test.go`
  - `TestFormatPlacementLegacyAllowURL`
  - `TestFormatPlacementLegacyAllowBody`
  - `TestFormatPlacementLegacyHeadersDefault`
  - `TestFormatPlacementAllowOnlyMixed`
  - `TestFormatPlacementAllowOnlyEmpty`
- `cmd/key-rest/main_test.go` (subprocess)
  - `TestVersionCommand`
  - `TestStatusCommandStopped`
  - `TestNoArgsShowsUsage`
  - `TestUnknownCommand`
  - `TestHelpCommand`
  - `TestListCommandStopped`
  - `TestStopWithoutRunningDaemon`
- `internal/crypto/crypto_test.go`
  - `TestEncryptDecrypt`
  - `TestDecryptWrongPassphrase`
  - `TestDecryptTooShort`
  - `TestEncryptDifferentCiphertexts`
  - `TestZeroClear`
  - `TestDeriveKeyDeterministic`
  - `TestDeriveKeyDifferentSalts`
  - `TestEncryptEmptyPlaintext`
  - `TestDecryptCorruptedData`
  - `TestMlockMunlockEmpty`
  - `TestMlockMunlockNonEmpty`
  - `TestZeroClearAndMunlockEmpty`
  - `TestZeroClearAndMunlockNonEmpty`
- `internal/daemon/daemon_test.go`
  - `TestNew`
  - `TestPidAndSocketPath`
  - `TestIsRunningNoPidFile`
  - `TestIsRunningInvalidPidFile`
  - `TestIsRunningStalePid`
  - `TestIsRunningCurrentProcess`
  - `TestStopNotRunning`
  - `TestStopMissingProcess`
  - `TestStopSendsSignal`
  - `TestReloadHandler`
  - `TestEnableHandler`
  - `TestStartAlreadyRunning`
  - `TestStartDecryptAllFails`
  - `TestStartFullLifecycle`
- `internal/daemon/process_attack_test.go`
  - `TestAttack_ProcMemCredentialExtraction`
  - `TestAttack_SIGQUITCrash`
  - `TestAttack_SIGKILLNoCleanup`
  - `TestAttack_PRSetDumpableNotSet`
  - `TestAttack_ProcEnvironLeak`
  - `TestAttack_DaemonProcMem`
- `internal/keystore/keystore_test.go`
  - `TestAddAndList`
  - `TestAddOverwrite`
  - `TestRemove`
  - `TestRemoveNotFound`
  - `TestDecryptAllAndLookup`
  - `TestDecryptAllWrongPassphrase`
  - `TestClearAll`
  - `TestFilePermissions`
  - `TestAddWhileDaemonRunning`
  - `TestDisable`
  - `TestEnable`
  - `TestListStatus`
  - `TestRemoveWhileDaemonRunning`
- `internal/keystore/keystore_extra_test.go`
  - `TestNewMkdirError`
  - `TestDefaultDirEnv`
  - `TestDefaultDirHome`
  - `TestLoadCorruptedFile`
  - `TestDecryptAllCorruptedFile`
  - `TestDecryptAllCorruptedEncryptedValue`
  - `TestRLockRUnlockDecrypted`
  - `TestSaveErrorOnUnwritableDir`
  - `TestEnableUnknownPrefix`
  - `TestEnableCorruptedFile`
  - `TestEnableInvalidEncryptedValue`
  - `TestLoadNonNotExistError`
  - `TestAddLoadCorruptedFile`
  - `TestAddReplaceWhileDecrypted`
  - `TestRemoveLoadCorruptedFile`
  - `TestRemoveSaveError`
  - `TestListLoadCorruptedFile`
  - `TestEnableWrongPassphrase`
- `internal/proxy/proxy_test.go`
  - `TestHandleBasicRequest`
  - `TestHandleKeyNotFound`
  - `TestHandleFieldRestrictionURL`
  - `TestHandleFieldRestrictionBody`
  - `TestHandleAllowURL`
  - `TestHandleAllowBody`
  - `TestHandleHTTPRejected`
  - `TestHandleURLPrefixMismatch`
  - `TestHasURLPrefix`
  - `TestHandleResponseMasking`
  - `TestHandleUserinfoRejected`
  - `TestHandleBase64TransformMasking`
  - `TestHandleJSONEscapedMasking`
  - `TestHandleInvalidType`
- `internal/proxy/allowonly_test.go`
  - `TestAllowOnlyHeader`
  - `TestAllowOnlyHeaderCaseInsensitive`
  - `TestAllowOnlyQuery`
  - `TestAllowOnlyField`
  - `TestAllowOnlyURLAndBody`
  - `TestAttack_AllowOnlyContentEmbedding`
  - `TestAllowOnlyMultipleHeaders`
  - `TestLegacyModeBackwardsCompat`
- `internal/proxy/attack_test.go`
  - `TestAttack_URLEncodedMaskingBypass`
  - `TestAttack_DoubleJSONEncodingBypass`
  - `TestAttack_CRLFInjectionInURL`
  - `TestAttack_PathTraversalPrefixBypass`
  - `TestAttack_SubstringMaskingCollision`
- `internal/proxy/gzip_attack_test.go`
  - `TestAttack_GzipMaskingBypass`
- `internal/proxy/coverage_test.go`
  - `TestParseRequestValid`
  - `TestParseRequestInvalid`
  - `TestProxyErrorString`
  - `TestToErrorResponseGenericError`
  - `TestNewProxyConstructor`
  - `TestCheckRedirectReturnsErrUseLastResponse`
  - `TestDecompressBodyGzip`
  - `TestDecompressBodyDeflate`
  - `TestDecompressBodyBrotli`
  - `TestDecompressBodyZstd`
  - `TestDecompressBodyIdentity`
  - `TestDecompressBodyUnknown`
  - `TestDecompressBodyMalformed`
  - `TestMaskPercentEncodedNoPercent`
  - `TestMaskPercentEncodedDecodesAndMasks`
  - `TestMaskPercentEncodedInvalidEscape`
  - `TestMaskPercentEncodedNoCredentialAfterDecode`
  - `TestContainsCRLFNone`
  - `TestIsValidHeaderNameRejects`
  - `TestHandleHTTPDoError`
  - `TestIsInAllowedQueryNoQueryString`
  - `TestIsInAllowedQueryParamWithoutEquals`
  - `TestIsInAllowedQueryFragmentStripped`
  - `TestIsInAllowedQueryRejectedParam`
  - `TestIsInAllowedFieldRejected`
  - `TestIsInAllowedFieldInvalidJSON`
  - `TestMaskCredentialsEmptyValueSkipped`
  - `TestRoundTripCRLFInResolvedValue`
  - `TestContainsCRLFNewline`
  - `TestIsValidHeaderNameByteRanges`
- `internal/proxy/headerinject_poc_test.go` — F-001 regression
  - `TestHeaderKeyInjectionRejected`
  - `TestMethodCRLFInjectionRawWire`
  - `TestURLCRLFInjectionRawWire`
- `internal/proxy/raceattack_poc_test.go` — F-002 regression
  - `TestDisableDuringRequestDoesNotLeak`
  - `TestDisableDuringRequestNaturalTimingDoesNotLeak`
- `internal/proxy/raceattack2_poc_test.go` — F-003 regression
  - `TestReloadAfterTamperDoesNotLeak`
- `internal/proxy/bodysmuggle_poc_test.go`
  - `TestTransferEncodingStrippedFromWire`
  - `TestValidationRaceDoesNotLeak`
- `internal/server/server_test.go`
  - `TestServerStartStop`
  - `TestServerHandleRequest`
  - `TestServerDisableEnable`
  - `TestServerInvalidJSON`
- `internal/server/server_extra_test.go`
  - `TestServerEmptyLineIgnored`
  - `TestServerVersion`
  - `TestServerReloadSuccess`
  - `TestServerReloadFailure`
  - `TestServerReloadNoHandler`
  - `TestServerEnableNoHandler`
  - `TestServerEnableFailure`
  - `TestServerDisableNoHandler`
  - `TestServerListNoHandler`
  - `TestServerStartDuplicateSocket`
  - `TestServerStartListenFails`
  - `TestServerConnectionSemFull`
  - `TestServerHTTPHandled`
- `internal/uri/uri_test.go`
  - `TestFindAllUnenclosed`
  - `TestFindAllEnclosed`
  - `TestFindAllEnclosedTransform`
  - `TestFindAllMultipleUnenclosed`
  - `TestFindAllMixed`
  - `TestFindAllNoMatch`
  - `TestReplaceUnenclosed`
  - `TestReplaceEnclosed`
  - `TestReplaceBase64Transform`
  - `TestReplaceMultipleUnenclosed`
  - `TestReplaceResolverError`
  - `TestReplaceNoMatch`
  - `TestReplaceUnknownTransform`
- `internal/uri/uri_bytes_test.go`
  - `TestReplaceBytesUnenclosed`
  - `TestReplaceBytesEnclosedTransform`
  - `TestReplaceBytesNoMatch`
  - `TestReplaceBytesResolverError`
  - `TestReplaceBytesUnknownTransform`
  - `TestReplaceBytesMultiArgsWithoutTransform`
  - `TestResolveMatchMultiArgsWithoutTransform`
  - `TestReplaceBytesPartialResolverFailure`
  - `TestResolveMatchExportedAlias`
  - `TestParseArgsMalformedString`
  - `TestParseArgsTrailingComma`
  - `TestParseArgsUnexpectedToken`
  - `TestParseArgsEmpty`
  - `TestFindAllEnclosedNonURI`
  - `TestZeroClear`
- `system-test/go/system_test.go`
  - `TestAllServices`
  - `TestResponseMasking`
  - `TestCompressionMasking`
  - `TestTruncatedKeyMasking`
  - `TestPercentEncodedMasking`
  - `TestPathTraversalBlocked`
  - `TestKeyDisableEnable`
