/-
 * Kernel-verified proofs about the key-rest credential masking pipeline.
 *
 * This file has two complementary layers of guarantee:
 *
 *   Part 1 — regression anchors (`masks_*`, proved by `decide`):
 *     Closed propositions that pin specific server-encoding scenarios for fixed
 *     test vectors.  These are kernel-verified test vectors, not general
 *     theorems.  In particular `masks_pct_b64` pins the percent+base64 bypass
 *     that was fixed in commit 45382b3.
 *
 *   Part 2 — universal theorems (`percent_roundtrip`, `masks_cred_universal`):
 *     `∀`-quantified statements proved by structural induction.  These promote
 *     the point checks to general guarantees where that is sound.
 *
 *   Part 2c — faithful decode model + bug detection:
 *     `queryUnescape` mirrors Go's all-or-nothing url.QueryUnescape (and its
 *     '+'→space).  `pipeline_buggy` (bail on decode failure) PROVES the v1.0.1
 *     leak (`buggy_leaks_stray_pct`); the fixed `pipeline` masks it
 *     (`fixed_masks_stray_pct`).  The same faithful model also surfaces a
 *     residual '+'→space miss in the shipped fix (`fixed_misses_plus_b64`).
 *

 * SCOPE (do not overstate — see lean/README.md §"What is NOT proved"):
 *   These proofs cover the MASKING LAYER only (response scanning).  key-rest's
 *   primary defence is placement validation (default-deny on where a credential
 *   may be sent); masking is a secondary block-list net.  A fully universal
 *   masking proof would still not mean "credentials never leak".
 *
 * Model notes:
 *   - Str = List Char, so every function is structurally recursive and the Lean
 *     kernel can reduce it (enabling `decide`).
 *   - replaceAll uses a Nat fuel parameter (= input length) for structural
 *     recursion, avoiding well-founded measures opaque to the kernel.
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

-- queryUnescape models Go's url.QueryUnescape faithfully: it is ALL-OR-NOTHING.
-- A single '%' not followed by two hex digits makes the whole decode fail
-- (returns none). It also turns '+' into a space. This is the partial decoder
-- whose failure path was the source of the masking bypass; percentDecodeAll
-- above is the tolerant decoder used as the fix's fallback.
def queryUnescape : Str → Option Str
  | '%' :: h :: l :: rest =>
    match hexDigit h, hexDigit l with
    | some hi, some lo =>
      match queryUnescape rest with
      | some d => some (Char.ofNat (hi * 16 + lo) :: d)
      | none   => none
    | _, _ => none
  | '%' :: _ => none
  | '+' :: rest =>
    match queryUnescape rest with
    | some d => some (' ' :: d)
    | none   => none
  | c :: rest =>
    match queryUnescape rest with
    | some d => some (c :: d)
    | none   => none
  | []        => some []

-- ---------------------------------------------------------------------------
-- JSON escaping (mirrors json.Marshal string escaping in proxy.go)
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
-- then the raw form.  Matches proxy.go maskCredentials.
def maskCredentials (cred uri body : Str) : Str :=
  let replacement := t "key-rest://" ++ uri
  let escaped     := jsonEscape cred
  let body        := if escaped != cred then replaceAll escaped replacement body else body
  replaceAll cred replacement body

-- maskTransformOutput: replaces one resolved transform output with its template.
def maskTransformOutput (b64cred template body : Str) : Str :=
  replaceAll b64cred template body

-- maskPercentEncoded models the FIXED proxy.go maskPercentEncoded (v1.0.2):
--   no '%'              -> unchanged
--   always use percentDecodeAll (tolerant, '+' kept as '+') — never
--   queryUnescape, which converts '+' to space and caused base64 credentials
--   containing '+' to be missed (fixed_misses_plus_b64).
def maskPercentEncoded (cred uri b64cred template body : Str) : Str :=
  if body.contains '%' then
    let decoded := percentDecodeAll body
    if decoded = body then body
    else
      let masked := maskCredentials cred uri (maskTransformOutput b64cred template decoded)
      if masked = decoded then body else masked
  else body

-- pipeline: the full masking pipeline for one response string.
-- b64cred = "" simulates "agent used raw key-rest://, no transform".
def pipeline (cred uri b64cred response : Str) : Str :=
  let template := t "{{ base64(key-rest://" ++ uri ++ t ") }}"
  let r := maskTransformOutput b64cred template response
  let r := maskCredentials cred uri r
  maskPercentEncoded cred uri b64cred template r

-- maskPercentEncoded_buggy models the OLD (pre-1.0.1) proxy.go: when the
-- all-or-nothing queryUnescape fails, it BAILS and returns the body UNMASKED.
-- This is the branch the earlier proof did not model, so it could not detect
-- the leak. Keeping it lets us prove the bug exists (buggy_leaks_stray_pct).
def maskPercentEncoded_buggy (cred uri b64cred template body : Str) : Str :=
  if body.contains '%' then
    match queryUnescape body with
    | none         => body  -- BUG: a single bad '%' disables masking entirely
    | some decoded =>
      if decoded = body then body
      else
        let masked := maskCredentials cred uri (maskTransformOutput b64cred template decoded)
        if masked = decoded then body else masked
  else body

def pipeline_buggy (cred uri b64cred response : Str) : Str :=
  let template := t "{{ base64(key-rest://" ++ uri ++ t ") }}"
  let r := maskTransformOutput b64cred template response
  let r := maskCredentials cred uri r
  maskPercentEncoded_buggy cred uri b64cred template r

-- ===========================================================================
-- Part 1 — regression anchors (kernel-verified test vectors, `decide`)
--
-- cred    = "ab~de"      (contains ~, non-alphanumeric)
-- b64cred = "YWJ+ZGU="  (base64("ab~de"); contains + and = for pct_b64)
-- uri     = "u/k"
-- ===========================================================================

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

-- ===========================================================================
-- Part 2a — universal theorem: percent round-trip
--
-- IMPORTANT FINDING.  The plan assumed `percentDecodeAll (percentEncodeAll s)
-- = s` is UNCONDITIONALLY true.  It is NOT, for this model:
--   - The Go test-server percent-encodes BYTES;  this model encodes CODEPOINTS
--     (`Char.toNat`).  For c.toNat ≥ 256, hexNibble (c.toNat / 16) overflows the
--     0..15 range and produces a non-hex char, so the round-trip breaks.
--   - Concrete break: a char with toNat = 256 encodes to "%G0", which does not
--     decode back.
-- The honest universal statement therefore carries a low-byte side condition
-- (every char < 256), which holds for all ASCII/Latin-1 credentials (real API
-- keys).  This gap is a model/Go divergence worth noting, not a proxy bug.
-- ===========================================================================

private theorem hexDigit_hexNibble (n : Nat) (h : n < 16) :
    hexDigit (hexNibble n) = some n := by
  have key : ∀ m : Fin 16, hexDigit (hexNibble m.val) = some m.val := by decide
  simpa using key ⟨n, h⟩

private theorem decode_cons_ne_pct (d : Char) (r : Str) (hd : d ≠ '%') :
    percentDecodeAll (d :: r) = d :: percentDecodeAll r := by
  rcases r with _ | ⟨x, _ | ⟨y, ys⟩⟩ <;> simp [percentDecodeAll, hd]

private theorem percentDecode_encodeChar (c : Char) (rest : Str) (hc : c.toNat < 256) :
    percentDecodeAll (encodeChar c ++ rest) = c :: percentDecodeAll rest := by
  unfold encodeChar
  split
  case isTrue h =>
    have hne : c ≠ '%' := by intro heq; rw [heq] at h; simp at h
    simpa using decode_cons_ne_pct c rest hne
  case isFalse h =>
    have hhi : c.toNat / 16 < 16 := by omega
    have hlo : c.toNat % 16 < 16 := by omega
    show percentDecodeAll ('%' :: hexNibble (c.toNat/16) :: hexNibble (c.toNat%16) :: rest)
       = c :: percentDecodeAll rest
    have step : percentDecodeAll ('%' :: hexNibble (c.toNat/16) :: hexNibble (c.toNat%16) :: rest)
        = (match hexDigit (hexNibble (c.toNat/16)), hexDigit (hexNibble (c.toNat%16)) with
           | some hi, some lo => Char.ofNat (hi*16+lo) :: percentDecodeAll rest
           | _, _ => '%' :: hexNibble (c.toNat/16) :: hexNibble (c.toNat%16) :: percentDecodeAll rest) := rfl
    rw [step, hexDigit_hexNibble _ hhi, hexDigit_hexNibble _ hlo]
    show Char.ofNat (c.toNat / 16 * 16 + c.toNat % 16) :: percentDecodeAll rest
       = c :: percentDecodeAll rest
    rw [show c.toNat / 16 * 16 + c.toNat % 16 = c.toNat by omega, Char.ofNat_toNat]

private def expand (f : Char → Str) : Str → Str
  | []      => []
  | c :: cs => f c ++ expand f cs

private theorem foldl_append_eq_expand (f : Char → Str) (s : Str) (acc : Str) :
    s.foldl (fun a c => a ++ f c) acc = acc ++ expand f s := by
  induction s generalizing acc with
  | nil => simp [expand]
  | cons c cs ih => simp [expand, ih, List.append_assoc]

private theorem percentEncodeAll_eq_expand (s : Str) :
    percentEncodeAll s = expand encodeChar s := by
  unfold percentEncodeAll; rw [foldl_append_eq_expand]; simp

private theorem percentDecode_expand (s : Str) (h : ∀ c ∈ s, c.toNat < 256) :
    percentDecodeAll (expand encodeChar s) = s := by
  induction s with
  | nil => rfl
  | cons c cs ih =>
    simp only [expand]
    rw [percentDecode_encodeChar c (expand encodeChar cs) (h c List.mem_cons_self)]
    rw [ih (fun x hx => h x (List.mem_cons_of_mem c hx))]

/-- Percent round-trip, universal over `s` under a low-byte side condition
    (every char < 256 — true for all ASCII/Latin-1 credentials). -/
theorem percent_roundtrip (s : Str) (h : ∀ c ∈ s, c.toNat < 256) :
    percentDecodeAll (percentEncodeAll s) = s := by
  rw [percentEncodeAll_eq_expand]; exact percentDecode_expand s h

-- ===========================================================================
-- Part 2b — universal theorem: the credential is masked in EVERY context.
--
-- Strategy: `maskCredentials` is `replaceAll cred (key-rest://uri) (...)` as its
-- OUTERMOST operation, and the pipeline's final result is `maskCredentials …`
-- in BOTH branches of `maskPercentEncoded`.  So if `replaceAll cred repl X`
-- never leaves `cred` as an infix (for any X), then neither does the pipeline,
-- for ANY uri / b64cred / response.
--
-- The absorption lemma `infix_replaceAll_false` is proved under `Disj cred repl`
-- (cred shares no character with the replacement).  This is a SUFFICIENT, not
-- necessary, condition (the truly necessary one is "no suffix of repl is a
-- prefix of cred"), chosen because it makes the seam-regeneration argument
-- tractable.  See Part 3 for why `replaceAll` needs such a side condition.
-- ===========================================================================

/-- Disjoint character sets: no char of `needle` occurs in `a`.
    `abbrev` so `Decidable` resolution sees the bounded-∀ and `decide` works. -/
private abbrev Disj (needle a : Str) : Prop := ∀ c, c ∈ needle → c ∉ a

private theorem and_true_left {a b : Bool} (h : (a && b) = true) : a = true := by
  cases a <;> simp_all
private theorem and_true_right {a b : Bool} (h : (a && b) = true) : b = true := by
  cases a <;> simp_all

-- Prepending a char-disjoint block cannot create a new infix of needle.
private theorem infix_append_disj (needle : Str) (hne : needle ≠ []) (b : Str) :
    ∀ a, Disj needle a → isInfix needle (a ++ b) = isInfix needle b := by
  intro a
  induction a with
  | nil => intro _; rfl
  | cons x xs ih =>
    intro hdisj
    rw [List.cons_append]
    have hpf : needle.isPrefixOf (x :: (xs ++ b)) = false := by
      cases needle with
      | nil => exact absurd rfl hne
      | cons n0 ns =>
        have hnx : n0 ≠ x := fun h => hdisj n0 (by simp) (by simp [h])
        simp [List.isPrefixOf, hnx]
    rw [show isInfix needle (x :: (xs ++ b))
          = (needle.isPrefixOf (x :: (xs ++ b)) || isInfix needle (xs ++ b)) from rfl,
        hpf, Bool.false_or]
    exact ih (fun c hc hcxs => hdisj c hc (List.mem_cons_of_mem x hcxs))

-- A char-disjoint, nonempty prefix cannot be a prefix of (repl ++ x).
private theorem prefix_disj_nil (q repl x : Str) (hrepl : repl ≠ [])
    (hdj : Disj q repl) (hq : q.isPrefixOf (repl ++ x) = true) : q = [] := by
  cases q with
  | nil => rfl
  | cons q0 qs =>
    exfalso
    cases repl with
    | nil => exact hrepl rfl
    | cons r0 rs =>
      rw [List.cons_append,
          show (q0 :: qs).isPrefixOf (r0 :: (rs ++ x))
             = ((q0 == r0) && qs.isPrefixOf (rs ++ x)) from rfl] at hq
      have heq : q0 = r0 := by have := and_true_left hq; simpa using this
      exact hdj q0 (by simp) (by rw [heq]; simp)

-- Any char-disjoint prefix of the replaced output is already a prefix of input.
private theorem prefix_replaceAllGo (needle repl : Str) (hrepl : repl ≠ []) :
    ∀ (n : Nat) (s q : Str), Disj q repl →
      q.isPrefixOf (replaceAllGo needle repl n s) = true → q.isPrefixOf s = true := by
  intro n
  induction n with
  | zero =>
    intro s q _ hq
    cases s with
    | nil => exact hq
    | cons c rest => exact hq
  | succ m ih =>
    intro s q hdj hq
    cases s with
    | nil => exact hq
    | cons c rest =>
      rw [show replaceAllGo needle repl (m+1) (c::rest)
            = (if needle = [] then (c::rest)
               else if needle.isPrefixOf (c::rest) then
                 repl ++ replaceAllGo needle repl m ((c::rest).drop needle.length)
               else c :: replaceAllGo needle repl m rest) from rfl] at hq
      by_cases h0 : needle = []
      · rw [if_pos h0] at hq; exact hq
      · rw [if_neg h0] at hq
        by_cases hp : needle.isPrefixOf (c::rest) = true
        · rw [if_pos hp] at hq
          have : q = [] := prefix_disj_nil q repl _ hrepl hdj hq
          subst this; rfl
        · rw [if_neg hp] at hq
          cases q with
          | nil => rfl
          | cons q0 qs =>
            rw [show (q0::qs).isPrefixOf (c :: replaceAllGo needle repl m rest)
                  = ((q0 == c) && qs.isPrefixOf (replaceAllGo needle repl m rest)) from rfl] at hq
            have h1 : (q0 == c) = true := and_true_left hq
            have h2 : qs.isPrefixOf (replaceAllGo needle repl m rest) = true := and_true_right hq
            have h3 : qs.isPrefixOf rest = true :=
              ih rest qs (fun d hd => hdj d (List.mem_cons_of_mem q0 hd)) h2
            show ((q0 == c) && qs.isPrefixOf rest) = true
            simp [h1, h3]

-- After a non-match, replacement of the tail cannot make needle a new prefix.
private theorem prefix_replaceAll_front (needle repl : Str) (hrepl : repl ≠ [])
    (c : Char) (rest : Str) (m : Nat)
    (hp : ¬ needle.isPrefixOf (c::rest) = true) (hdisj : Disj needle repl) :
    needle.isPrefixOf (c :: replaceAllGo needle repl m rest) = false := by
  cases needle with
  | nil => exact absurd rfl hp
  | cons n0 ns =>
    by_cases hn0 : n0 = c
    · cases hpref : (n0 :: ns).isPrefixOf (c :: replaceAllGo (n0::ns) repl m rest) with
      | false => rfl
      | true =>
        exfalso
        rw [show (n0::ns).isPrefixOf (c :: replaceAllGo (n0::ns) repl m rest)
              = ((n0 == c) && ns.isPrefixOf (replaceAllGo (n0::ns) repl m rest)) from rfl] at hpref
        have htail : ns.isPrefixOf (replaceAllGo (n0::ns) repl m rest) = true := and_true_right hpref
        have hns_disj : Disj ns repl := fun d hd => hdisj d (List.mem_cons_of_mem n0 hd)
        have hnsrest : ns.isPrefixOf rest = true :=
          prefix_replaceAllGo (n0::ns) repl hrepl m rest ns hns_disj htail
        have hb : (n0 == c) = true := by simp [hn0]
        have hcontra : (n0::ns).isPrefixOf (c::rest) = true := by
          show ((n0 == c) && ns.isPrefixOf rest) = true
          simp [hb, hnsrest]
        exact hp hcontra
    · simp [List.isPrefixOf, hn0]

-- Main absorption lemma: replaceAll removes every occurrence of a nonempty
-- needle whose characters are disjoint from the (nonempty) replacement.
private theorem infix_replaceAllGo_false (needle repl : Str)
    (hne : needle ≠ []) (hrepl : repl ≠ []) (hdisj : Disj needle repl) :
    ∀ (n : Nat) (s : Str), s.length ≤ n →
      isInfix needle (replaceAllGo needle repl n s) = false := by
  have hempty : needle.isEmpty = false := by
    cases needle with | nil => exact absurd rfl hne | cons => rfl
  intro n
  induction n with
  | zero =>
    intro s hs
    have hs0 : s = [] := by
      cases s with | nil => rfl | cons a t => exfalso; simp only [List.length_cons] at hs; omega
    subst hs0
    rw [show isInfix needle (replaceAllGo needle repl 0 []) = needle.isEmpty from rfl]
    exact hempty
  | succ m ih =>
    intro s hs
    cases s with
    | nil =>
      rw [show isInfix needle (replaceAllGo needle repl (m+1) []) = needle.isEmpty from rfl]
      exact hempty
    | cons c rest =>
      rw [show replaceAllGo needle repl (m+1) (c::rest)
            = (if needle = [] then (c::rest)
               else if needle.isPrefixOf (c::rest) then
                 repl ++ replaceAllGo needle repl m ((c::rest).drop needle.length)
               else c :: replaceAllGo needle repl m rest) from rfl]
      rw [if_neg hne]
      by_cases hp : needle.isPrefixOf (c::rest) = true
      · rw [if_pos hp, infix_append_disj needle hne _ repl hdisj]
        apply ih
        have hd : ((c::rest).drop needle.length).length = (rest.length + 1) - needle.length := by
          rw [List.length_drop]; rfl
        have hnl : needle.length ≠ 0 := by
          cases needle with | nil => exact absurd rfl hne | cons => simp
        rw [hd]; simp only [List.length_cons] at hs; omega
      · rw [if_neg hp,
            show isInfix needle (c :: replaceAllGo needle repl m rest)
              = (needle.isPrefixOf (c :: replaceAllGo needle repl m rest)
                 || isInfix needle (replaceAllGo needle repl m rest)) from rfl]
        have hrest : isInfix needle (replaceAllGo needle repl m rest) = false := by
          apply ih; simp only [List.length_cons] at hs; omega
        have hfront : needle.isPrefixOf (c :: replaceAllGo needle repl m rest) = false :=
          prefix_replaceAll_front needle repl hrepl c rest m hp hdisj
        simp [hfront, hrest]

private theorem infix_replaceAll_false (needle repl s : Str)
    (hne : needle ≠ []) (hrepl : repl ≠ []) (hdisj : Disj needle repl) :
    isInfix needle (replaceAll needle repl s) = false := by
  unfold replaceAll
  exact infix_replaceAllGo_false needle repl hne hrepl hdisj s.length s (Nat.le_refl _)

private theorem repl_ne (uri : Str) : (t "key-rest://" ++ uri) ≠ [] := by
  simp [t]

private theorem maskCredentials_cred_free (cred uri X : Str)
    (hne : cred ≠ []) (hdisj : Disj cred (t "key-rest://" ++ uri)) :
    isInfix cred (maskCredentials cred uri X) = false := by
  unfold maskCredentials
  exact infix_replaceAll_false cred (t "key-rest://" ++ uri) _ hne (repl_ne uri) hdisj

/-- Universal masking guarantee for the credential.
    For ANY uri / b64cred / response, if the credential is nonempty and shares
    no character with its replacement template, then it never appears verbatim
    in the pipeline output.  This is strictly stronger than the fixed-vector
    `masks_raw`: it quantifies over all four inputs.  (Note: `"ab~de"` does NOT
    satisfy `Disj` because it shares `'e'` with "key-rest://" — see the examples
    below — so this technique covers a disjoint-charset credential, while
    `"ab~de"` is covered pointwise by the Part 1 anchors.) -/
theorem masks_cred_universal (cred uri b64cred response : Str)
    (hne : cred ≠ []) (hdisj : Disj cred (t "key-rest://" ++ uri)) :
    isInfix cred (pipeline cred uri b64cred response) = false := by
  have key : ∀ X, isInfix cred (maskCredentials cred uri X) = false :=
    fun X => maskCredentials_cred_free cred uri X hne hdisj
  unfold pipeline maskPercentEncoded
  dsimp only
  -- Every leaf is either the argument body (= maskCredentials cred uri _) or a
  -- fresh maskCredentials cred uri _ — all cred-free by `key`.
  split
  · split
    · exact key _
    · split
      · exact key _
      · exact key _
  · exact key _

-- A concrete credential whose characters are disjoint from the template, so the
-- universal theorem applies.  (Acceptance: NoReform/Disj instance via `decide`.)
example : Disj (t "ab~dq") (t "key-rest://u/k") := by decide
-- The realistic Part-1 vector "ab~de" does NOT satisfy Disj (it shares 'e'),
-- which is why the universal proof uses a disjoint credential and "ab~de" stays
-- a pointwise anchor.  This documents the sufficient-vs-necessary gap.
example : ¬ Disj (t "ab~de") (t "key-rest://u/k") := by decide

-- ===========================================================================
-- Part 2c — the faithful model now DETECTS the percent-decode bypass (v1.0.1).
--
-- The earlier model used a tolerant, total decoder and OMITTED the "decode
-- failed -> return unmasked" branch, so it could not see the bug. With
-- queryUnescape (all-or-nothing) and maskPercentEncoded_buggy (bail on
-- failure) modeled faithfully, the leak is now a provable fact, and the fix
-- is verified against the same input.
--
-- Input: cred "ab~de" percent-encoded as "ab%7Ede", followed by a stray
-- invalid "%g" (mimicking a literal '%' in an error body). queryUnescape
-- fails on "%g", which in the old code disabled masking entirely.
-- ===========================================================================

/-- Faithful OLD model LEAKS: the credential is recoverable from the output
    (after the agent percent-decodes it). `= true` is the leak. -/
theorem buggy_leaks_stray_pct :
    let cred := t "ab~de"
    let uri  := t "u/k"
    let resp := t "ab%7Ede%g"
    isInfix cred (percentDecodeAll (pipeline_buggy cred uri (t "") resp)) = true := by
  decide

/-- FIXED model masks the same input: the stray '%' no longer disables masking. -/
theorem fixed_masks_stray_pct :
    let cred := t "ab~de"
    let uri  := t "u/k"
    let resp := t "ab%7Ede%g"
    isInfix cred (percentDecodeAll (pipeline cred uri (t "") resp)) = false := by
  decide

/-- v1.0.2 fix: always using percentDecodeAll closes the '+'-as-space bypass.
    '+' is now kept as '+', so "YWJ+ZGU%3D" decodes to "YWJ+ZGU=" which
    matches b64cred and is masked. -/
theorem fixed_masks_plus_b64 :
    let cred    := t "ab~de"
    let b64cred := t "YWJ+ZGU="
    let uri     := t "u/k"
    let resp    := t "YWJ+ZGU%3D"    -- '+' literal, '=' percent-encoded as %3D
    isInfix b64cred (percentDecodeAll (pipeline cred uri b64cred resp)) = false := by
  decide

-- ===========================================================================
-- Model fidelity notes (what this model does and does NOT mirror)
--
-- Now modeled faithfully (this is what made the bug provable):
--   * queryUnescape: Go url.QueryUnescape's all-or-nothing failure AND '+'→space.
--   * maskPercentEncoded / _buggy: the decode-failure branch (fix vs old bail).
--
-- Still simplified (documented, not yet modeled):
--   * maskCredentials models ONE credential; proxy.go sorts MANY longest-first.
--     That longest-first pass is why two overlapping keys can partially mask
--     each other (observed in live testing).
--   * maskTruncatedKeys (regex, OpenAI/Stripe/localhost only) is omitted; it is
--     an EXTRA masker, so omitting it is conservative for these leak proofs.
--   * Only the response BODY is modeled; proxy.go runs the same sequence on
--     response HEADERS, so the guarantees transfer unchanged.
--   * percentEncodeAll encodes by codepoint; Go encodes by byte (see Part 2a).
-- ===========================================================================

-- ===========================================================================
-- Part 3 — limits that are NOT universally provable (documented, not proved)
--
-- (a) `∀ cred`: FALSE.  A credential equal to a substring of the replacement
--     template "key-rest://"++uri (e.g. "key", "rest", "://", or uri text)
--     survives masking.  Fixing the credential is not incidental — the ∀ is
--     genuinely false.  `Disj cred repl` rules these out by construction.
--
-- (b) `∀ encoding` (the adversary's encoding space): FALSE by mechanism.
--     Masking is an enumerated block-list: raw / json-esc / base64 / percent /
--     percent+base64 / truncated.  Anything outside the list slips through —
--     base64url, hex, ROT-N, DOUBLE percent-encoding (the proxy decodes only
--     one layer; see `masks_pct_b64`, which is single-layer), inter-character
--     spacing, one-char-at-a-time spelling, stream splitting, …  A block-list
--     is inherently incomplete, so no `∀ encoding` theorem can hold.
--
-- (c) Free-form LLM response text: containment is impossible.  A generative
--     text field is effectively full-alphabet and unbounded; neither an
--     allow-list nor a block-list constrains it.
--
-- (d) `replaceAll` seam regeneration: the single-pass `replaceAll` does not
--     rescan the seam between an inserted `replacement` and following text, so
--     `∀ cred body. isInfix cred (replaceAll cred repl body) = false` is FALSE
--     in general.  Minimal counterexample:
--         replaceAll "ab" "a" "abb" = "a" ++ replaceAll "ab" "a" "b"
--                                    = "a" ++ "b" = "ab"
--     ("ab" ∉ "a", yet it reappears at the seam.)  This is exactly why
--     `infix_replaceAll_false` requires the `Disj`/border-free side condition.
--     See lean/README.md for the design decision (keep current replaceAll +
--     side condition vs. rescanning replaceAll).
-- ===========================================================================

#print axioms percent_roundtrip
#print axioms infix_replaceAll_false
#print axioms masks_cred_universal
#print axioms masks_raw
#print axioms masks_pct_b64
#print axioms buggy_leaks_stray_pct
#print axioms fixed_masks_stray_pct
#print axioms fixed_masks_plus_b64
