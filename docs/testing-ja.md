[English](testing.md) | [Japanese](testing-ja.md)

# テスト構成

key-rest のテストは 3 種類に分かれます: パッケージごとの単体テスト、複数言語にまたがるシステムテスト、確認済みのセキュリティ findings に対する PoC / regression テスト。

## パッケージごとの単体テスト

`internal/` と `cmd/` 配下の各 Go パッケージに `*_test.go` ファイルが置かれています。標準の `go test` ワークフローで動き、外部依存はありません。

| Package | 場所 | 範囲 |
|---|---|---|
| `internal/crypto` | `crypto_test.go` | AES-256-GCM round-trip、PBKDF2 鍵導出、mlock / zero-clear プリミティブ |
| `internal/keystore` | `keystore_test.go`, `keystore_extra_test.go` | `Add` / `Remove` / `DecryptAll` / `Lookup` / `Disable` / `Enable` / `ListStatus`、on-disk フォーマット、ファイルパーミッション、エラーパス |
| `internal/uri` | `uri_test.go`, `uri_bytes_test.go` | `FindAll`, `Replace`, `ReplaceBytes`、transform 関数、`parseArgs` のエッジケース |
| `internal/proxy` | `proxy_test.go`, `coverage_test.go`, `headerinject_poc_test.go`, `raceattack_poc_test.go`, `raceattack2_poc_test.go`, `bodysmuggle_poc_test.go` | リクエスト validation、URI 解決、レスポンス masking、secure-transport の wire 組み立て、F-001 / F-002 / F-003 regression |
| `internal/server` | `server_test.go`, `server_extra_test.go` | Unix socket プロトコル dispatch (`http` / `reload` / `enable` / `disable` / `list` / `version`)、接続上限、malformed 入力の扱い |
| `internal/daemon` | `daemon_test.go` | `IsRunning`、`Start` / `Stop` ライフサイクル (in-test SIGTERM trap)、`reload` / `enable` ハンドラ |
| `cmd/key-rest` | `main_pure_test.go`, `main_test.go` | `formatPlacement` などの pure helper、ビルド済みバイナリに対する subprocess テスト (`version` / `status` / 未知コマンド / `help` / daemon 未起動時の `stop`) |

実行:

```bash
make test-go       # 全パッケージ、失敗時即停止
make coverage      # text サマリ
make coverage-html # 閲覧可能な HTML レポート (coverage.html)
```

## システムテスト

クロス言語の end-to-end テストは `system-test/` 配下にあります。実際の `key-rest` daemon を起動し、bundled サービスの credential をすべて登録し、bundled `test-server` に対して全サポート対象クライアント (curl, Go, Python, Node.js) を駆動します。詳細は [`system-test/README-ja.md`](../system-test/README-ja.md)。

| Suite | 場所 | 実行ドライバ |
|---|---|---|
| Curl | `system-test/curl/system-test.sh` | bash |
| Go | `system-test/go/system_test.go` | `go test` |
| Python | `system-test/python/system_test.py` | python3 |
| Node.js | `system-test/node/system_test.mjs` | node |

全 suite を一括実行:

```bash
make test-system
```

各 suite は `make build` 済みのバイナリと test-server の証明書が必要です。

## セキュリティ PoC / regression テスト

`internal/proxy/` 配下の `*_poc_test.go` は、SPECA メソドロジー監査の findings ([`audit-speca-ja.md`](audit-speca-ja.md)) で発見された脆弱性の PoC として始まりました。修正適用後にアサーションを反転し、現在は regression として機能しています:

- `headerinject_poc_test.go` — F-001 (Header Key CRLF Injection)
- `raceattack_poc_test.go` — F-002 (Disable/Mask race)。決定論的な gated 版と 200 試行の natural timing 版あり (後者は `-short` でスキップ)
- `raceattack2_poc_test.go` — F-003 (Reload + `keys.enc` 改竄 race)
- `bodysmuggle_poc_test.go` — 補助 (Transfer-Encoding strip、validation race)

race detector で隠れたデータ競合がないことも検証:

```bash
go test -race -short ./internal/proxy
```

## CI

`.github/workflows/test.yml` が `main` への push と pull request 毎に単体テストを実行し、coverage サマリを出力、`coverage.out` を build artifact としてアップロードします。

## test-server

`test-server/` はサポート対象のサードパーティサービス (OpenAI, Anthropic, Stripe 等) をすべてモックする localhost HTTPS サーバです。システムテストおよび (証明書経由で間接的に) ハッキングチャレンジ環境から消費されます。サーバ自身に `*_test.go` ファイルはありません — それを駆動するシステムテスト suite で検証されます。

## テストツリー

プロジェクト内の全 `Test*` 関数をファイル別にまとめたもの。

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
