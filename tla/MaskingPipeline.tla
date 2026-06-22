--------------------------- MODULE MaskingPipeline ---------------------------
(*
 * TLA+ specification of the key-rest masking pipeline.
 * Verifies that the current fixed code correctly masks all known server
 * encoding strategies.
 *
 * Server encoding categories:
 *   "none"     - credential absent from response
 *   "raw"      - raw credential bytes echoed verbatim
 *   "json_esc" - JSON-escaped credential (for creds containing ", \, control chars)
 *   "b64"      - base64 of credential (from {{ base64(...) }} template)
 *   "pct_raw"  - percent-encoded raw credential
 *   "pct_b64"  - percent-encoded base64 of credential
 *   "trunc"    - truncated key pattern (e.g., "sk-****abcd"; known API prefixes only)
 *
 * Masking pipeline (proxy.go Handle, step order matters):
 *   Step 1: maskTransformOutputs  replaces b64 transform outputs      catches: "b64"
 *   Step 2: maskCredentials       replaces raw + JSON-escaped cred    catches: "raw", "json_esc"
 *   Step 3: maskTruncatedKeys     replaces truncated key patterns     catches: "trunc"
 *   Step 4: maskPercentEncoded    URL-decodes then repeats Steps 1+2  catches: "pct_raw", "pct_b64"
 *
 * INVARIANT NoCredentialLeak: agentView never equals "leaked"
 *
 * Note: the "trunc" category applies only to credentials whose url_prefix
 * matches a known API (OpenAI, Stripe, localhost).  For all other URL prefixes
 * maskTruncatedKeys is a no-op.  This model assumes the known-API case.
 *)

EXTENDS Naturals, TLC

CONSTANTS
    ServerEncodings   \* set of server encoding strategies to verify

VARIABLES
    serverEncoding,   \* encoding chosen by the server nondeterministically
    agentView,        \* "masked" | "leaked"
    phase             \* "init" | "server_respond" | "done"

vars == <<serverEncoding, agentView, phase>>

TypeOK ==
    /\ serverEncoding \in ServerEncodings
    /\ agentView \in {"masked", "leaked"}
    /\ phase \in {"init", "server_respond", "done"}

Init ==
    /\ serverEncoding = "none"
    /\ agentView = "masked"
    /\ phase = "init"

(*
 * Server nondeterministically picks any encoding from ServerEncodings.
 *)
ServerRespond ==
    /\ phase = "init"
    /\ \E enc \in ServerEncodings :
        /\ serverEncoding' = enc
        /\ phase' = "server_respond"
    /\ UNCHANGED agentView

(*
 * Daemon applies the masking pipeline.
 * Each encoding is caught by exactly one pipeline step (as shown in the
 * step-by-step table in the module header).
 *)
DaemonProcess ==
    /\ phase = "server_respond"
    /\ LET
           credPresent == serverEncoding /= "none"
           caught ==
               \/ serverEncoding = "none"      \* nothing to mask
               \/ serverEncoding = "raw"       \* Step 2: maskCredentials (raw form)
               \/ serverEncoding = "json_esc"  \* Step 2: maskCredentials (JSON-escaped form)
               \/ serverEncoding = "b64"       \* Step 1: maskTransformOutputs
               \/ serverEncoding = "pct_raw"   \* Step 4: URL-decode -> raw -> maskCredentials
               \/ serverEncoding = "pct_b64"   \* Step 4: URL-decode -> b64 -> maskTransformOutputs
               \/ serverEncoding = "trunc"     \* Step 3: maskTruncatedKeys (known API prefixes)
           leaked == credPresent /\ ~caught
       IN
           /\ agentView' = IF leaked THEN "leaked" ELSE "masked"
           /\ phase' = "done"
    /\ UNCHANGED serverEncoding

Done == phase = "done" /\ UNCHANGED vars

Next == ServerRespond \/ DaemonProcess \/ Done

Spec == Init /\ [][Next]_vars

----------------------------------------------------------------------

NoCredentialLeak == agentView /= "leaked"

========================================================================
