# 8B Fleeting — HTTPS as Multi-Property Protocol; CSRF as the Residual Gap

**Source:** OWASP CSRF Prevention Cheat Sheet (`cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html`) + Computerphile-style HTTP/TLS background video
**Read:** 2026-04-26
**Consolidates into:** No `formal_notes.tex` write-up (per Part 8 plan: STRIDE/security formalization is post-exam). This file *is* the consolidation; thesis-relevant tangents → `interests.md` per 8C.
**Status:** populated through conversation 2026-04-26 (not scaffold-and-defer)

---

## 1. The TLS property split — exam core

TLS realises three of the six STRIDE-mitigation properties **simultaneously**, all scoped to the **channel** between browser and server:

| TLS mechanism | Property | STRIDE threat mitigated |
|---|---|---|
| Certificate chain verified against trust roots | Authentication (of the *server*) | Spoofing |
| MAC on each TLS record + signed handshake transcript | Integrity (of bytes in flight) | Tampering |
| Symmetric session key encrypts records | Confidentiality (of bytes in flight) | Information Disclosure |

Why the slides spend time on this: TLS is the cleanest example in the deck of one protocol bundling several STRIDE properties at once. For any data-flow element crossing a trust boundary, "use HTTPS" closes S+T+I in one move.

**What TLS does NOT do (the seam):** TLS authenticates the *server* to the *client*. It does **not** authenticate the *client* to the *server* — that is bolted on at the application layer, almost always via a session cookie. TLS also does not authenticate the *initiator* of a request — only the destination endpoint.

---

## 2. CSRF as the residual gap

**The mechanism in one sentence:** the browser attaches cookies based on the request's *destination*, not its *initiator*; so a page on `evil.com` can cause the browser to send a perfectly authenticated, perfectly TLS-encrypted request to `bank.com` carrying the user's session cookie, with no protocol-level signal that the user did not consent.

**The structural gap:** TLS protects the *channel*; CSRF attacks the *server's belief about consent*. Same authenticated session, different intent. Two distinct concerns, no single protocol covers both.

**Same-Origin Policy asymmetry (the load-bearing detail):** SOP blocks `evil.com`'s JS from *reading* `bank.com` responses, but does not block it from *making* requests to `bank.com`. CSRF lives in that asymmetry — it doesn't need to read; it just needs the side effect to happen.

---

## 3. Defenses, by who supplies the missing signal

Each defense adds a signal the cookie+TLS pair doesn't carry: *"is this request really coming from my own page?"* Three families:

- **Server-issued secret (application layer).** Synchronizer token, signed double-submit cookie. Server emits a per-session secret embedded in its own pages; legitimate POST echoes it back; `evil.com` can't read it (SOP). Invariant: *"requests carry a secret only same-origin pages could have read."*
- **Browser-supplied metadata.** `Origin` / `Referer` header check; custom headers + CORS preflight; Fetch Metadata. The browser tells the server which page initiated the request, and JS cannot forge these. Invariant: *"trust the browser as a witness to request origin."*
- **Browser-side enforcement.** `SameSite` cookie attribute. Browser refuses to attach the cookie at all on cross-site requests. Invariant: *"the credential never leaves its origin context."*

Defense in depth = stack across families. Each fails independently (token leaked via XSS; header stripped by proxy; old browser without SameSite). No single layer is fatal if others hold.

---

## 4. Course frame — why this pair sits in the syllabus

**Slide-level (Level 1 + 2):**
- Populates the STRIDE table: CSRF straddles *Spoofing* (forged request appears to be from the user) and *Tampering* (unauthorized state change). Single threat → multiple STRIDE categories → multiple mitigation properties. Concrete demonstration of the per-element / per-interaction matrices from 8A.
- Slide narrative: *here is a protocol (TLS) that gives you S+T+I for free at the channel; here is a threat (CSRF) that lives in the gap that protocol does not close; here is the vocabulary (STRIDE) that lets you find such gaps systematically.*

**STPA-Sec / Young-Leveson framing (Level 3 — deferred):**

CSRF, viewed through STPA-Sec rather than STRIDE, is a **control-structure inadequacy**, not a component bug. The implicit safety constraint *"state-changing requests must reflect user intent"* is unenforced because the browser-server control loop has no channel for intent — only for authentication. No single component is faulty; the hazard emerges from the composition. Each CSRF defense is a control-structure modification adding a feedback signal so the server's process model gains "did the user actually want this?" alongside "is this user authenticated?".

This is the same critique Leveson levels at the chain model in Ch. 2. CSRF is, structurally, a textbook STAMP-style hazard wearing security clothes.

→ Logged for `interests.md` (8C entry): CSRF as the cleanest small worked case of "STPA-Sec rides the same γ as STPA" — Young & Leveson's central claim, in a 50-line example. Defenses-as-additional-observation-channels is the coalgebraic margin reading. Formal write-up post-exam.

---

## 5. Avizienis cross-link

In the formal vocabulary of `formal_notes.tex` (`sec:dependability-attributes`): CSRF is an **In ∧ Av_auth** violation with a compositional twist. The attacker rides on a valid authentication context (Av_auth holding for the session) to violate integrity (In) of state-changing operations. The interesting bit for the chain model: there is no single $t_0$ to point at — no component is in a fault state. This is precisely a P1-violation (in the vocabulary of `sec:preconditions`) for the security setting: empty fault set, hazard arises from interaction. STAMP/STPA-Sec is the natural framework; the chain model bottoms out.

(Margin observation only — do not unfold; the formal write-up of STRIDE is post-exam.)
