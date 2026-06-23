--------------------------- MODULE MaskingPipeline ---------------------------
(*
 * TLA+ specification of the key-rest masking pipeline.
 * Reworked for non-vacuity, concurrent TOCTOU, step ordering, placement gate.
 *
 * Lean vs. TLA+ roles
 * -------------------
 * Lean proves: string masking functions are correct for all strings/credentials
 *              (infinite domain, structural induction, decide).
 * TLA+ verifies: pipeline orchestration — step coverage (non-vacuous), step
 *                ordering (load-bearing), concurrent snapshot TOCTOU, and the
 *                placement gate invariant.  The same encoding scenarios as
 *                Lean's Part 1 regression anchors are also verified here so
 *                the two systems can be directly compared.
 *
 * Non-vacuity design
 * ------------------
 * caught is derived from step coverage sets (Covered), NOT by listing
 * ServerEncodings.  ServerEncodings is the adversary's encoding choice set;
 * Covered is what the pipeline actually handles.  Any encoding in
 * ServerEncodings \ Covered makes TLC report NoCredentialLeak violated.
 *
 * Universality
 * ------------
 * TLC explores ALL combinations of:
 *   serverEncoding  every element of ServerEncodings (adversary's nondeterminism)
 *   apiKnown        both TRUE and FALSE (known / unknown API prefix)
 *   placement       both "header" and "body" (agent's placement choice)
 *   DisableKey      can interleave with ANY pipeline phase after CheckAllowed,
 *                   covering all possible concurrent disable/reload races
 *
 * Phase order (matches proxy.go Handle)
 * --------------------------------------
 *   init
 *     -> check_place  (placement gate; reject if placement not allowed)
 *     -> snap         (check liveHasCred; reject if key disabled in tiny window)
 *     -> server_resp  (server echoes credential in some encoding)
 *     -> s1 -> s2 -> s3 -> s4  (masking pipeline steps)
 *     -> done
 *   rejected          (terminal: placement bad or key disabled at snap time)
 *
 * DisableKey is enabled from check_place onwards (concurrent via Unix socket).
 * If it fires before TakeSnapshot, liveHasCred=FALSE at snap -> RejectFromSnap.
 * If it fires after TakeSnapshot, snapshotHasCred stays TRUE; the snapshot
 * ensures masking is unaffected (UseSnapshot=TRUE).
 *
 * Server encoding categories
 * --------------------------
 *  "none"         credential absent from response
 *  "raw"          credential bytes verbatim                    (Lean: masks_raw)
 *  "json_esc"     JSON-escaped credential                      (Lean: masks_json_esc)
 *  "b64"          base64(credential) from template             (Lean: masks_b64)
 *  "pct_raw"      percent-encoded raw credential               (Lean: masks_pct_raw)
 *  "pct_b64"      percent-encoded base64(credential)           (Lean: masks_pct_b64)
 *  "pct_stray"    pct-encoded cred + stray '%'                 (Lean: buggy_leaks/fixed_masks_stray_pct)
 *  "pct_plus_b64" base64 cred with '+', other byte pct-encoded (Lean: fixed_masks_plus_b64)
 *  "trunc"        truncated key (known API prefix only)
 *  + any extras in ServerEncodings for negative/non-vacuity testing
 *
 * Masking pipeline steps (proxy.go Handle, order is load-bearing)
 * ----------------------------------------------------------------
 *  Step 1: maskTransformOutputs   catches: "b64"
 *  Step 2: maskCredentials        catches: "raw", "json_esc"
 *  Step 3: maskTruncatedKeys      catches: "trunc" (known API only)
 *  Step 4: maskPercentEncoded     catches: "pct_raw","pct_b64"
 *                                  + ExtraStep4Catches (empty=buggy, full=v1.0.2)
 *
 * Configuration constants (vary by cfg file)
 * -------------------------------------------
 *  ServerEncodings    adversary's encoding choice set
 *  ExtraStep4Catches  {} = buggy decoder (pre-v1.0.2)
 *                     {"pct_stray","pct_plus_b64"} = fixed decoder (v1.0.2)
 *  UseSnapshot        TRUE = correct TOCTOU (snapshot); FALSE = naive (live set)
 *  Step4Enabled       TRUE = normal; FALSE = step 4 omitted (disorder test)
 *
 * Invariants
 * ----------
 *  TypeOK                 type correctness
 *  NoCredentialLeak       agent never sees raw credential (excl. IntentionalLimit)
 *  ResolvedImpliesAllowed placement gate: credential forwarded only to allowed placement
 *)

EXTENDS Naturals, TLC

CONSTANTS
    ServerEncodings,    \* adversary's encoding set (see cfg files)
    ExtraStep4Catches,  \* extra encodings the fixed step-4 decoder catches
    UseSnapshot,        \* TRUE = use snapshotHasCred; FALSE = naive liveHasCred
    Step4Enabled        \* TRUE = run step 4; FALSE = omit (disorder test)

VARIABLES
    serverEncoding,     \* encoding the server chose
    credVisible,        \* TRUE = credential still visible in current body
    agentView,          \* "masked" | "leaked"
    phase,              \* pipeline phase (see above)
    snapshotHasCred,    \* TRUE = credential was live at TakeSnapshot time (write-once)
    liveHasCred,        \* TRUE = credential currently in live key set (mutable)
    apiKnown,           \* TRUE = url_prefix matches known API (enables trunc masking)
    placement           \* "header" | "body" — where agent placed the placeholder

vars == <<serverEncoding, credVisible, agentView, phase,
          snapshotHasCred, liveHasCred, apiKnown, placement>>

\* ---------------------------------------------------------------------------
\* Step coverage sets (derived from mechanism, NOT from ServerEncodings)
\* ---------------------------------------------------------------------------

Step1Catches == {"b64"}
Step2Catches == {"raw", "json_esc"}
Step3Catches(ak) == IF ak THEN {"trunc"} ELSE {}
BaseStep4Catches == {"pct_raw", "pct_b64"}

\* EffectiveStep4Catches: base catches + fixes, or empty if step 4 is omitted.
EffectiveStep4Catches ==
    IF Step4Enabled THEN BaseStep4Catches \cup ExtraStep4Catches ELSE {}

\* Covered: full set of encodings the current pipeline configuration handles.
Covered(ak) ==
    Step1Catches \cup Step2Catches \cup Step3Catches(ak) \cup EffectiveStep4Catches

\* EffectiveCred: the credential-existence flag the masking steps read.
\* UseSnapshot=TRUE (correct): reads snapshotHasCred (immutable after TakeSnapshot).
\* UseSnapshot=FALSE (naive): reads liveHasCred (DisableKey can clear it mid-flight).
EffectiveCred == IF UseSnapshot THEN snapshotHasCred ELSE liveHasCred

\* ---------------------------------------------------------------------------
\* Type invariant
\* ---------------------------------------------------------------------------

TypeOK ==
    /\ serverEncoding \in ServerEncodings \cup {"none"}
    /\ credVisible \in BOOLEAN
    /\ agentView \in {"masked", "leaked"}
    /\ phase \in {"init", "check_place", "snap", "rejected",
                  "server_resp", "s1", "s2", "s3", "s4", "done"}
    /\ snapshotHasCred \in BOOLEAN
    /\ liveHasCred \in BOOLEAN
    /\ apiKnown \in BOOLEAN
    /\ placement \in {"header", "body"}

\* ---------------------------------------------------------------------------
\* Initial state
\* ---------------------------------------------------------------------------

Init ==
    /\ serverEncoding = "none"
    /\ credVisible = FALSE
    /\ agentView = "masked"
    /\ phase = "init"
    /\ snapshotHasCred = FALSE
    /\ liveHasCred = TRUE          \* credential starts as live
    /\ apiKnown \in BOOLEAN        \* TLC explores both known and unknown API
    /\ placement \in {"header", "body"}  \* TLC explores both placements

\* ---------------------------------------------------------------------------
\* Actions
\* ---------------------------------------------------------------------------

\* Agent submits request; daemon begins processing.
RequestSubmitted ==
    /\ phase = "init"
    /\ phase' = "check_place"
    /\ UNCHANGED <<serverEncoding, credVisible, agentView,
                   snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Placement gate PASS: allowed placement -> proceed to snapshot.
CheckAllowed ==
    /\ phase = "check_place"
    /\ placement = "header"       \* key's allowedPlacement = "header"
    /\ phase' = "snap"
    /\ UNCHANGED <<serverEncoding, credVisible, agentView,
                   snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Placement gate FAIL: forbidden placement -> reject.
RejectRequest ==
    /\ phase = "check_place"
    /\ placement /= "header"
    /\ phase' = "rejected"
    /\ UNCHANGED <<serverEncoding, credVisible, agentView,
                   snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Snapshot + liveness check.  The daemon takes a snapshot of the live key
\* set immediately before forwarding.  If the key is already disabled (tiny
\* TOCTOU window between check_place and snap), the request is also rejected.
TakeSnapshot ==
    /\ phase = "snap"
    /\ liveHasCred = TRUE         \* key must still be live to proceed
    /\ snapshotHasCred' = TRUE    \* snapshot records: credential was live here
    /\ phase' = "server_resp"
    /\ UNCHANGED <<serverEncoding, credVisible, agentView, liveHasCred, apiKnown, placement>>

\* Key disabled in the tiny window after CheckAllowed but before TakeSnapshot.
RejectFromSnap ==
    /\ phase = "snap"
    /\ liveHasCred = FALSE
    /\ phase' = "rejected"
    /\ UNCHANGED <<serverEncoding, credVisible, agentView,
                   snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Server nondeterministically picks an encoding.  "none" = absent; any other
\* encoding = credential present in that form.  snapshotHasCred=TRUE here
\* (guaranteed by TakeSnapshot), so the credential was definitely in the request.
ServerRespond ==
    /\ phase = "server_resp"
    /\ \E enc \in ServerEncodings :
        /\ serverEncoding' = enc
        /\ credVisible' = (enc /= "none")
        /\ phase' = "s1"
    /\ UNCHANGED <<agentView, snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Step 1: maskTransformOutputs — catches "b64"
ApplyStep1 ==
    /\ phase = "s1"
    /\ credVisible' = IF serverEncoding \in Step1Catches /\ EffectiveCred
                      THEN FALSE ELSE credVisible
    /\ phase' = "s2"
    /\ UNCHANGED <<serverEncoding, agentView, snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Step 2: maskCredentials — catches "raw", "json_esc"
ApplyStep2 ==
    /\ phase = "s2"
    /\ credVisible' = IF serverEncoding \in Step2Catches /\ EffectiveCred
                      THEN FALSE ELSE credVisible
    /\ phase' = "s3"
    /\ UNCHANGED <<serverEncoding, agentView, snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Step 3: maskTruncatedKeys — catches "trunc" when apiKnown=TRUE only.
\* When apiKnown=FALSE the truncated form is NOT caught: IntentionalLimit (not a bug).
ApplyStep3 ==
    /\ phase = "s3"
    /\ credVisible' = IF serverEncoding \in Step3Catches(apiKnown) /\ EffectiveCred
                      THEN FALSE ELSE credVisible
    /\ phase' = "s4"
    /\ UNCHANGED <<serverEncoding, agentView, snapshotHasCred, liveHasCred, apiKnown, placement>>

\* Step 4: maskPercentEncoded — URL-decodes (tolerant decoder, v1.0.2) then
\* re-applies steps 1+2.  When Step4Enabled=FALSE, EffectiveStep4Catches={},
\* so nothing is caught (disorder test).
ApplyStep4 ==
    /\ phase = "s4"
    /\ LET newVisible == IF serverEncoding \in EffectiveStep4Catches /\ EffectiveCred
                         THEN FALSE ELSE credVisible
       IN /\ credVisible' = newVisible
          /\ agentView' = IF newVisible THEN "leaked" ELSE "masked"
          /\ phase' = "done"
    /\ UNCHANGED <<serverEncoding, snapshotHasCred, liveHasCred, apiKnown, placement>>

\* DisableKey: removes the credential from the live set at any time from
\* check_place onwards, simulating concurrent disable/reload via the Unix socket.
\*
\* Design: DisableKey writes liveHasCred only.  snapshotHasCred is write-once
\* (set only by TakeSnapshot) and is unaffected.
\*
\*   UseSnapshot=TRUE (correct):  steps read snapshotHasCred=TRUE; DisableKey
\*     after TakeSnapshot does not affect masking.  NoCredentialLeak holds.
\*   UseSnapshot=FALSE (naive):   steps read liveHasCred; DisableKey between
\*     TakeSnapshot and any step makes that step skip masking -> LEAK.
\*     TLC reports NoCredentialLeak violated (MaskingPipeline_naive.cfg).
DisableKey ==
    /\ phase \in {"snap", "server_resp", "s1", "s2", "s3", "s4"}
    /\ liveHasCred = TRUE
    /\ liveHasCred' = FALSE
    /\ UNCHANGED <<serverEncoding, credVisible, agentView, phase,
                   snapshotHasCred, apiKnown, placement>>

Done == phase \in {"done", "rejected"} /\ UNCHANGED vars

Next ==
    \/ RequestSubmitted
    \/ CheckAllowed
    \/ RejectRequest
    \/ TakeSnapshot
    \/ RejectFromSnap
    \/ ServerRespond
    \/ ApplyStep1
    \/ ApplyStep2
    \/ ApplyStep3
    \/ ApplyStep4
    \/ DisableKey
    \/ Done

Spec == Init /\ [][Next]_vars

\* ---------------------------------------------------------------------------
\* Invariants
\* ---------------------------------------------------------------------------

\* Intentional design limit: maskTruncatedKeys only applies to known API prefixes.
\* When apiKnown=FALSE and the server echoes a truncated key, masking does not
\* fire.  This is a documented scope limit (not a bug).
IntentionalLimit == serverEncoding = "trunc" /\ ~apiKnown

\* Core safety: agent never sees the raw credential in the response.
\* The only permitted "leaks" are IntentionalLimits (trunc + unknown API).
NoCredentialLeak == agentView /= "leaked" \/ IntentionalLimit

\* Placement gate: once past check_place (phases after snap), placement was allowed.
\* TLC verifies RejectRequest correctly blocks all bad-placement paths.
ResolvedImpliesAllowed ==
    phase \in {"server_resp", "s1", "s2", "s3", "s4", "done"} =>
        placement = "header"

========================================================================
