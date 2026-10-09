# Generative engine optimization (GEO) audit

Checked 2026-10-09 against official guidance and the live https://potpuri.cc/ site.
GEO means making useful public information discoverable and understandable in AI
search experiences. It is not a separate guaranteed ranking mechanism.

## Current official guidance

- [Google: AI features and your website](https://developers.google.com/search/docs/appearance/ai-features)
  says existing SEO fundamentals apply. Eligibility requires indexing and snippet
  eligibility. No AI text file, special markup, or special schema is required.
  Priorities are crawl access, internal links, textual content, page experience,
  and structured data matching visible content. Inclusion is not guaranteed.
- [OpenAI: crawler documentation](https://developers.openai.com/api/docs/bots)
  distinguishes OAI-SearchBot (search) from GPTBot (possible model training).
  These controls are independent. ChatGPT-User represents user-triggered visits;
  robots.txt may not apply to those visits. Check published crawler IP ranges if
  infrastructure blocks otherwise-allowed requests.
- [llms.txt proposal](https://llmstxt.org/) specifies a Markdown H1, short
  blockquote summary, optional explanatory text, and H2 sections containing
  descriptive links. It recommends linked Markdown resources and discoverability
  through `rel="describedby"`. This is a proposal, not a requirement for Google
  AI search, a replacement for robots.txt/sitemaps, or a promise of citations.

## Added in this change

- `web/public/llms.txt`: concise, factual product overview and public references.
- `web/public/product-guide.md`: human- and agent-readable product facts and answers.
- `web/src/layouts/Base.astro`: a discovery link to `/llms.txt`.

No private account data, secrets, user profiles, internal APIs, development
roadmaps, or unimplemented email features are advertised as available. Prices and
catalogue totals are intentionally omitted because they change. Recheck these
files when access, membership, demo, or privacy behavior changes.

## SEO improvements implemented after the audit

- Canonical origin is `https://potpuri.cc`; shared metadata normalizes public URLs
  to trailing-slash paths and drops query strings.
- `@astrojs/sitemap` generates sitemap-index.xml and child sitemaps for actual
  built public homepage, legal, and book pages. robots.txt uses an absolute URL.
- Account, login/signup, demo, quote shells, and the generic reader shell use
  noindex/follow and are excluded from the sitemap.
- Open Graph and Twitter summary metadata share each page's title, description,
  canonical URL, and the existing 512px app icon.
- The homepage includes WebSite structured data. Public book pages include Book
  data with only title, author, and URL; no invented reviews or offers.
- Build-time book metadata now paginates the free catalogue rather than fetching
  only the first 24 books. The public production API is the default build source;
  PUBLIC_API_URL can override it. Each request times out after ten seconds.
  Guest snapshots remain a fallback during outages, and failures log a warning.
- Book pages have specific title/description metadata and a no-JavaScript notice.
  Full public book overview/landing-page content remains a separate opportunity.
- Run `cd web && npm run build && npm run test:seo` to check generated output.

## Original findings and recommended next work

1. **Broken sitemap declaration.** Live `/robots.txt` advertises
   `/sitemap-index.xml`, which returned HTTP 404. No sitemap integration appears
   in `web/astro.config.mjs`. Generate a real sitemap with absolute URLs, include
   only intended public canonical pages, and publish an absolute Sitemap URL.
2. **No canonical tags.** The shared layout currently has no canonical link.
   Establish the `https://potpuri.cc` origin, normalize trailing slashes and
   parameterized previews, and emit appropriate canonical URLs.
3. **Book pages lack public static book information.** The sampled live
   `/read/gutenberg-46` HTML has reader/dialog scaffolding but no substantive
   rendered book overview. Add public server-rendered title, author, description,
   and useful links, or dedicated book landing pages; keep the reader UI separate
   if that makes the information more accessible.
4. **Clarify indexing boundaries.** Existing wildcard robots rules disallow
   `/app/` and `/v1/` but allow the rest. Keep personal account surfaces protected
   by authentication. Consider noindex for login/signup and account/profile
   shells; robots disallow alone is neither noindex nor a security boundary.
   Decide search-crawler versus training-crawler access explicitly before adding
   bot-specific rules. A bot-specific group must preserve the intended path
   exclusions because matching groups do not inherit wildcard rules.
5. **Structured data is optional, not an AI requirement.** Once public page
   content is reliable, consider WebSite/WebApplication on the homepage and Book
   on dedicated book pages, only with facts visible there. Do not invent reviews,
   prices, publisher details, or claim special GEO benefits.
6. **Trust and policy accuracy.** Legal pages are marked working drafts. The
   privacy page contains `privacy@bookworm.example` and makes export/erasure
   statements that need verification against implemented capabilities. Replace
   placeholder contacts and resolve policy/product mismatches before treating
   these pages as authoritative launch policies.
7. **Measure rather than promise.** Verify ownership in Google Search Console,
   submit the working sitemap, inspect public-page indexing, and review search
   traffic. Google reports AI feature traffic within Web search performance, not
   as a standalone GEO guarantee. Review crawler access logs without exposing
   reading history or adding behavioral tracking.

Training permissions, legal promises, and private account authorization remain
unchanged. robots.txt only updates its sitemap declaration; wildcard access and
existing account/API path exclusions remain as before. Canonical and metadata
changes do not alter navigation or redirect behavior.
