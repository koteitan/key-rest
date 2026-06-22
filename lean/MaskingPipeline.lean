/-
 * Formal model of key-rest response masking pipeline.
 *
 * Uses List Char (not String) so every function is structurally recursive
 * and the Lean kernel can reduce them — enabling `decide` (kernel-verified).
 *
 * replaceAll uses a fuel parameter (= input length) to achieve structural
 * recursion on Nat, since the well-founded (s.length) measure is opaque to
 * the kernel.
 *
 * Proves:
 *   1. Buggy maskPercentEncoded leaks base64 through percent-encoding.
 *   2. Fixed maskPercentEncoded blocks the leak.
 *
 * Test credential "ab~de" → base64 "YWJ+ZGU=" (contains '+' %2B and '=' %3D).
 -/

abbrev Str := List Char

-- Convert a string literal to Str
private abbrev t (lit : String) : Str := lit.toList

-- ---------------------------------------------------------------------------
-- replaceAll
-- Structural recursion on Nat fuel (fuel = input length is always sufficient).
-- The kernel reduces this fully for any concrete input.
-- ---------------------------------------------------------------------------

private def replaceAllGo (needle replacement : Str) : Nat → Str → Str
  | _,     []          => []
  | 0,     r           => r  -- fuel exhausted (never reached with fuel = r.length)
  | n + 1, r@(c :: rest) =>
    if needle = [] then r
    else if needle.isPrefixOf r then
      replacement ++ replaceAllGo needle replacement n (r.drop needle.length)
    else
      c :: replaceAllGo needle replacement n rest

private def replaceAll (needle replacement s : Str) : Str :=
  replaceAllGo needle replacement s.length s

-- ---------------------------------------------------------------------------
-- isInfix: true iff needle is a contiguous subsequence of haystack.
-- Structural recursion on haystack.
-- ---------------------------------------------------------------------------

private def isInfix (needle : Str) : Str → Bool
  | []            => needle.isEmpty
  | s@(_ :: rest) => needle.isPrefixOf s || isInfix needle rest

-- ---------------------------------------------------------------------------
-- Percent-encoding (matches test-server/main.go percentEncodeAll).
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
-- Masking (modeled after internal/proxy/proxy.go)
-- ---------------------------------------------------------------------------

def maskCredentials (cred uri body : Str) : Str :=
  replaceAll cred (t "key-rest://" ++ uri) body

def maskTransformOutput (b64cred template body : Str) : Str :=
  replaceAll b64cred template body

-- ---------------------------------------------------------------------------
-- Buggy pipeline (proxy.go before fix):
-- URL-decodes, then calls maskCredentials only — maskTransformOutput is skipped.
-- ---------------------------------------------------------------------------

def maskPercentEncoded_buggy (cred uri body : Str) : Str :=
  let decoded := percentDecodeAll body
  let masked  := maskCredentials cred uri decoded
  if masked != decoded then masked else body

def pipeline_buggy (cred uri b64cred response : Str) : Str :=
  let template := t "{{ base64(key-rest://" ++ uri ++ t ") }}"
  let r := maskTransformOutput b64cred template response
  let r := maskCredentials cred uri r
  maskPercentEncoded_buggy cred uri r

-- ---------------------------------------------------------------------------
-- Fixed pipeline (proxy.go after fix):
-- URL-decodes, then applies maskTransformOutput AND maskCredentials.
-- ---------------------------------------------------------------------------

def maskPercentEncoded_fixed (cred uri b64cred template body : Str) : Str :=
  let decoded := percentDecodeAll body
  let masked  := maskTransformOutput b64cred template decoded
  let masked  := maskCredentials cred uri masked
  if masked != decoded then masked else body

def pipeline_fixed (cred uri b64cred response : Str) : Str :=
  let template := t "{{ base64(key-rest://" ++ uri ++ t ") }}"
  let r := maskTransformOutput b64cred template response
  let r := maskCredentials cred uri r
  maskPercentEncoded_fixed cred uri b64cred template r

-- ---------------------------------------------------------------------------
-- Theorems
--
-- cred    = "ab~de"
-- b64cred = "YWJ+ZGU="   (base64 of "ab~de"; contains '+' and '=')
-- percentEncodeAll "YWJ+ZGU=" = "YWJ%2BZGU%3D"
--   ('+' → %2B,  '=' → %3D — exercises the non-'=' case the old model missed)
-- ---------------------------------------------------------------------------

/-- Buggy pipeline leaks: the attacker URL-decodes the response to recover b64cred. -/
theorem buggy_leaks_base64 :
    let cred       := t "ab~de"
    let b64cred    := t "YWJ+ZGU="
    let uri        := t "u/k"
    let serverResp := percentEncodeAll b64cred   -- "YWJ%2BZGU%3D"
    let result     := pipeline_buggy cred uri b64cred serverResp
    isInfix b64cred (percentDecodeAll result) = true := by
  decide

/-- Fixed pipeline is safe: b64cred cannot be recovered by URL-decoding the result. -/
theorem fixed_masks_base64 :
    let cred       := t "ab~de"
    let b64cred    := t "YWJ+ZGU="
    let uri        := t "u/k"
    let serverResp := percentEncodeAll b64cred
    let result     := pipeline_fixed cred uri b64cred serverResp
    isInfix b64cred (percentDecodeAll result) = false ∧
    isInfix cred result = false := by
  decide
