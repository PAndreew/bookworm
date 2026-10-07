# Legal posture

Written for you, not for users. The user-facing text lives in `web/src/pages/legal/`.

I am not a lawyer and this is not legal advice. Have a qualified lawyer in your
operating jurisdiction review the terms before launch.

## 1. The uploader warranty you asked for

It is in the terms: publishers warrant that they hold the rights, and they indemnify
Bookworm. That clause is standard, worth having, and does real work — **between you
and the uploader**. It gives you a contractual claim against someone who lies to you.

What it does **not** do, in the EU:

- **It does not transfer platform liability.** A rights holder sues the platform, not
  your uploader. An indemnity means you can try to recover afterwards; it does not
  stop you being the defendant.
- **DSA (Regulation 2022/2065)** applies regardless of what your terms say. As a
  hosting service you need a working notice-and-action mechanism (Art. 16), you must
  give reasons for removals (Art. 17), and you must suspend repeat infringers (Art. 23).
- **CDSM Art. 17** is the harder one. A platform that stores and gives the public
  access to a large amount of user-uploaded protected work performs an *act of
  communication to the public* itself. The safe-harbour reflex of "we are just a host"
  does not hold for this shape of product. You need either a licence, or to show best
  efforts to obtain one plus expeditious takedown on notice.

**This is why the takedown endpoint and the `strikes` counter are in v1** rather than
a later milestone. They are cheap now and expensive to retrofit, and shipping
publisher uploads without them would leave a real exposure.

### What I would add before publisher uploads go live to the public
1. Human verification of publisher identity before the first publish — `publishers.verified_at` exists and is unset until someone checks.
2. A review queue: upload lands in `state='review'`, not `'published'`.
3. A counter-notice path, so a wrongly-removed publisher can respond.
4. A named contact point for notices (DSA Art. 11/12).

## 2. Age: 13+

Per your decision, the minimum age is 13, declared at signup and enforced twice —
in the handler and by a `CHECK` constraint on `users.birthdate`.

What that choice buys you:
- **COPPA** (US) applies to under-13s. A 13+ floor keeps you out of the verifiable-parental-consent regime entirely.
- **GDPR Art. 8**: the digital-consent age is 16 by default, but member states may lower it to 13. Germany sets **16**. So a 13-year-old in Germany still needs parental consent for consent-based processing.

**Mitigation, already in the design:** the lawful basis for the core service is
*contract* (Art. 6(1)(b)), not consent, and there is no profiling, no behavioural
advertising, and no public-by-default profile. Default visibility is `friends`, which
is what keeps a minor's reading from being world-readable on signup.

If you later add behavioural advertising or recommendations that profile minors, this
analysis changes and you will need a per-country age gate. Worth deciding before,
not after.

Note also the **DSA Art. 28** duty: a platform accessible to minors must take
appropriate measures for their privacy and safety. Private-by-default and block/report
are the baseline; both are in v1.

## 3. Public-domain content

Project Gutenberg texts are public domain **in the United States**. Copyright terms
differ by country: a work that is free in the US may still be in copyright in the EU,
which generally runs life + 70 years.

The starter shelf is chosen to be safe in both the US and the EU — every author died
before 1955. Two to watch if you extend the shelf:

- *The Great Gatsby* (Fitzgerald, d. 1940) — EU-clear since 2011, US-clear since 2021.
- Anything by an author who died after 1955 is **not** EU public domain even if Gutenberg hosts it.

The Gutenberg licence header and footer are stripped at ingest, which is both a
product decision and a trademark one: the "Project Gutenberg" mark has its own terms,
and the cleanest position is not to redistribute their branding at all.

## 4. Data protection

- Passwords: Argon2id, never recoverable.
- TOTP secrets: AES-GCM encrypted at rest, so a dumped `users` table does not hand over everyone's second factor.
- Sessions: opaque random tokens, SHA-256 hashed in the database. A leak of the sessions table does not yield usable cookies.
- `users.deleted_at` exists for soft delete. **Still to build**: a real erasure path (Art. 17) that hard-deletes or anonymises reading history, and an export endpoint (Art. 20). Flagged honestly — both are v2.
- Reading history is sensitive. What someone reads can reveal health, sexuality, religion, or politics. It is not special-category data by definition, but treat it as close to it: no third-party analytics on reading events.
