/-
 * Kernel-verified proofs that the current key-rest masking pipeline
 * correctly masks all known server encoding scenarios.
 *
 * Uses List Char so every function is structurally recursive and the Lean
 * kernel can reduce them, enabling `decide` (kernel-verified proof).
 *
 * replaceAll uses a fuel parameter (= input length) for structural recursion
 * on Nat, avoiding well-founded measures that are opaque to the kernel.
 *
 * Encoding scenarios proved safe:
 *   1. raw:      credential echoed verbatim
 *   2. json_esc: credential echoed JSON-escaped (e.g., newline -> backslash-n)
 *   3. b64:      base64 transform output echoed verbatim
 *   4. pct_raw:  percent-encoded raw credential
 *   5. pct_b64:  percent-encoded base64 output (vulnerability fixed in 45382b3)
 *
 * Test credentials:
 *   "ab~de"  (~ is non-alphanumeric; exercises percent-encoding)
 *             base64 = "YWJ+ZGU=" (contains + and = for pct_b64 scenario)
 *   "ab\nde" (contains newline; exercises JSON-escaped scenario)
 -/

abbrev Str := List Char
private abbrev t (lit : String) : Str := lit.toList

-- ---------------------------------------------------------------------------
-- replaceAll: structural recursion on Nat fuel (fuel = input length)
-- ---------------------------------------------------------------------------

private def replaceAllGo (needle replacement : Str) : Nat → Str → Str
  | _,     []          => []
  | 0,     r           => r  -- fuel exhausted; unreachable when fuel = r.length
  | n + 1, r@(c :: rest) =>
    if needle = [] then r
    else if needle.isPrefixOf r then
      replacement ++ replaceAllGo needle replacement n (r.drop needle.length)
    else
      c :: replaceAllGo needle replacement n rest

private def replaceAll (needle replacement s : Str) : Str :=
  replaceAllGo needle replacement s.length s

-- ---------------------------------------------------------------------------
-- isInfix: true iff needle is a contiguous sub-sequence of haystack
-- ---------------------------------------------------------------------------

private def isInfix (needle : Str) : Str → Bool
  | []            => needle.isEmpty
  | s@(_ :: rest) => needle.isPrefixOf s || isInfix needle rest

-- ---------------------------------------------------------------------------
-- Percent-encoding (matches test-server/main.go percentEncodeAll)
-- Every non-alphanumeric character is encoded as %HH (uppercase hex).
-- ---------------------------------------------------------------------------

private def hexNibble (n : Nat) : Char :=
  if n < 10 then Char.ofNat ('0'.toNat + n)
  else       Char.ofNat ('A'.toNat + n - 10)

private def encodeChar (c : Char) : Str :=
  if c.isAlpha || c.isDigit then [c]
  else ['%', hexNibble (c.toNat / 16), hexNibble (c.toNat % 16)]

def percentEncodeAll (s : Str) : Str :=
  s.foldl (fun acc c => acc ++ encodeChar c) []

private def hexDigit (c : Char) : Option Nat :=
  if '0' ≤ c && c ≤ '9' then some (c.toNat - '0'.toNat)
  else if 'A' ≤ c && c ≤ 'F' then some (c.toNat - 'A'.toNat + 10)
  else if 'a' ≤ c && c ≤ 'f' then some (c.toNat - 'a'.toNat + 10)
  else none

def percentDecodeAll : Str → Str
  | '%' :: h :: l :: rest =>
    match hexDigit h, hexDigit l with
    | some hi, some lo => Char.ofNat (hi * 16 + lo) :: percentDecodeAll rest
    | _,       _       => '%' :: h :: l :: percentDecodeAll rest
  | c :: rest => c :: percentDecodeAll rest
  | []        => []

-- ---------------------------------------------------------------------------
-- JSON escaping (mirrors json.Marshal string escaping in proxy.go)
-- Covers the characters that produce a different JSON representation.
-- ---------------------------------------------------------------------------

private def jsonEscapeChar (c : Char) : Str :=
  if      c == '"'  then ['\\', '"']
  else if c == '\\' then ['\\', '\\']
  else if c == '\n' then ['\\', 'n']
  else if c == '\r' then ['\\', 'r']
  else if c == '\t' then ['\\', 't']
  else [c]

private def jsonEscape (s : Str) : Str :=
  s.foldl (fun acc c => acc ++ jsonEscapeChar c) []

-- ---------------------------------------------------------------------------
-- Masking (models internal/proxy/proxy.go)
-- ---------------------------------------------------------------------------

-- maskCredentials: replaces the JSON-escaped form first (more specific),
-- then the raw form.  Matches the behaviour of proxy.go maskCredentials.
def maskCredentials (cred uri body : Str) : Str :=
  let replacement := t "key-rest://" ++ uri
  let escaped     := jsonEscape cred
  let body        := if escaped != cred then replaceAll escaped replacement body else body
  replaceAll cred replacement body

-- maskTransformOutput: replaces one resolved transform output with its template.
def maskTransformOutput (b64cred template body : Str) : Str :=
  replaceAll b64cred template body

-- maskPercentEncoded: URL-decodes the body, then re-applies both masking steps.
-- If masking the decoded form produced a change, the decoded+masked form is
-- returned; otherwise the original is kept.  (Models proxy.go maskPercentEncoded.)
def maskPercentEncoded (cred uri b64cred template body : Str) : Str :=
  let decoded := percentDecodeAll body
  let masked  := maskTransformOutput b64cred template decoded
  let masked  := maskCredentials cred uri masked
  if masked != decoded then masked else body

-- pipeline: the full masking pipeline for one response string.
-- b64cred = "" simulates "agent used raw key-rest://, no transform".
def pipeline (cred uri b64cred response : Str) : Str :=
  let template := t "{{ base64(key-rest://" ++ uri ++ t ") }}"
  let r := maskTransformOutput b64cred template response
  let r := maskCredentials cred uri r
  maskPercentEncoded cred uri b64cred template r

-- ---------------------------------------------------------------------------
-- Theorems: all five encoding scenarios are safely masked.
--
-- cred    = "ab~de"      (contains ~, non-alphanumeric)
-- b64cred = "YWJ+ZGU="  (base64("ab~de"); contains + and = for pct_b64)
-- uri     = "u/k"
-- ---------------------------------------------------------------------------

/-- Scenario 1: credential appears verbatim in the response. -/
theorem masks_raw :
    let cred    := t "ab~de"
    let b64cred := t "YWJ+ZGU="
    let uri     := t "u/k"
    let result  := pipeline cred uri b64cred cred
    isInfix cred result = false := by
  decide

/-- Scenario 2: credential contains a newline; server returns the JSON-escaped form. -/
theorem masks_json_esc :
    let cred    := t "ab\nde"
    let uri     := t "u/k"
    let escaped := jsonEscape cred               -- "ab\nde" -> "ab\\nde"
    let result  := pipeline cred uri (t "") escaped
    isInfix cred result = false ∧
    isInfix escaped result = false := by
  decide

/-- Scenario 3: base64 transform output appears verbatim in the response. -/
theorem masks_b64 :
    let cred    := t "ab~de"
    let b64cred := t "YWJ+ZGU="
    let uri     := t "u/k"
    let result  := pipeline cred uri b64cred b64cred
    isInfix b64cred result = false := by
  decide

/-- Scenario 4: percent-encoded raw credential (e.g., test-server /percent-echo/). -/
theorem masks_pct_raw :
    let cred    := t "ab~de"
    let uri     := t "u/k"
    let resp    := percentEncodeAll cred         -- "ab%7Ede"
    let result  := pipeline cred uri (t "") resp
    isInfix cred result = false := by
  decide

/-- Scenario 5: percent-encoded base64 output.
    The bug (fixed in 45382b3) was that maskPercentEncoded did not call
    maskTransformOutput after decoding, so "YWJ%2BZGU%3D" slipped through. -/
theorem masks_pct_b64 :
    let cred    := t "ab~de"
    let b64cred := t "YWJ+ZGU="
    let uri     := t "u/k"
    let resp    := percentEncodeAll b64cred      -- "YWJ%2BZGU%3D"
    let result  := pipeline cred uri b64cred resp
    isInfix b64cred (percentDecodeAll result) = false ∧
    isInfix cred result = false := by
  decide
