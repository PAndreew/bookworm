-- Bookworm v1 schema.
--
-- Design note: pages are immutable once an edition is published. Likes and highlights
-- anchor to page ids forever, so re-chunking a book means a NEW edition -- never an
-- UPDATE of existing pages. See docs/DECISIONS.md.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

-- ----------------------------------------------------------------- users & auth

CREATE TYPE tier         AS ENUM ('free', 'plus', 'pro');
CREATE TYPE visibility   AS ENUM ('public', 'friends', 'private');
CREATE TYPE account_role AS ENUM ('reader', 'publisher', 'admin');

CREATE TABLE users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email           citext NOT NULL UNIQUE,
    handle          citext NOT NULL UNIQUE CHECK (handle ~ '^[a-z0-9_]{3,24}$'),
    display_name    text   NOT NULL DEFAULT '',
    password_hash   text   NOT NULL,            -- argon2id, PHC string format
    birthdate       date   NOT NULL,            -- 13+ gate, also enforced at signup
    timezone        text   NOT NULL DEFAULT 'UTC',  -- IANA name; day boundary for streaks
    role            account_role NOT NULL DEFAULT 'reader',
    tier            tier   NOT NULL DEFAULT 'free',
    visibility      visibility NOT NULL DEFAULT 'friends',
    email_verified  boolean NOT NULL DEFAULT false,
    totp_secret     bytea,                      -- NULL = 2FA off
    totp_enabled_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz
);

-- Hard floor on age, belt-and-braces with the application-level check.
ALTER TABLE users ADD CONSTRAINT users_min_age_13
    CHECK (birthdate <= (CURRENT_DATE - INTERVAL '13 years'));

CREATE TABLE totp_recovery_codes (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash bytea NOT NULL,                   -- sha256, single use
    used_at   timestamptz
);
CREATE INDEX ON totp_recovery_codes (user_id) WHERE used_at IS NULL;

CREATE TABLE sessions (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,          -- sha256 of the opaque cookie token
    -- A session that passed the password but not yet TOTP is "pending": it may only
    -- call /auth/2fa/verify. This is what stops 2FA from being bypassable.
    mfa_pending boolean NOT NULL DEFAULT false,
    user_agent  text NOT NULL DEFAULT '',
    ip          inet,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz
);
CREATE INDEX ON sessions (user_id) WHERE revoked_at IS NULL;

-- ----------------------------------------------------------------- catalog

CREATE TYPE license_kind  AS ENUM ('public_domain', 'licensed');
CREATE TYPE edition_state AS ENUM ('draft', 'review', 'published', 'taken_down');

CREATE TABLE publishers (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    name          text NOT NULL,
    -- Set only after a human checks them. The uploader warranty in the terms does not
    -- make this optional in the EU -- see docs/LEGAL.md.
    verified_at   timestamptz,
    strikes       int  NOT NULL DEFAULT 0,      -- repeat-infringer policy
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE books (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         text NOT NULL UNIQUE,
    title        text NOT NULL,
    author       text NOT NULL,
    description  text NOT NULL DEFAULT '',
    cover_url    text,
    language     text NOT NULL DEFAULT 'en',
    license      license_kind NOT NULL,
    publisher_id uuid REFERENCES publishers(id) ON DELETE RESTRICT,  -- NULL for our own PD ingests
    source_ref   text,                          -- e.g. 'gutenberg:1342'
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON books (license);

-- An edition pins one chunking of one source text. Immutable once published.
CREATE TABLE book_editions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    book_id         uuid NOT NULL REFERENCES books(id) ON DELETE CASCADE,
    chunker_version int  NOT NULL,
    state           edition_state NOT NULL DEFAULT 'draft',
    page_count      int  NOT NULL DEFAULT 0,
    word_count      int  NOT NULL DEFAULT 0,
    published_at    timestamptz,
    taken_down_at   timestamptz,
    takedown_reason text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (book_id, chunker_version)
);

-- At most one published edition per book, enforced by the database rather than hoped for.
CREATE UNIQUE INDEX one_published_edition_per_book
    ON book_editions (book_id) WHERE state = 'published';

CREATE TABLE pages (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_id     uuid NOT NULL REFERENCES book_editions(id) ON DELETE CASCADE,
    idx            int  NOT NULL,               -- 0-based, stable forever
    chapter_title  text,
    content        text NOT NULL,
    char_count     int  NOT NULL,
    content_sha256 bytea NOT NULL,              -- idempotent, verifiable ingest
    UNIQUE (edition_id, idx)
);

-- ----------------------------------------------------------------- reading & metering

CREATE TABLE reading_progress (
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    edition_id    uuid NOT NULL REFERENCES book_editions(id) ON DELETE CASCADE,
    last_page_idx int NOT NULL DEFAULT 0,
    furthest_idx  int NOT NULL DEFAULT 0,
    is_private    boolean NOT NULL DEFAULT false,  -- hide this one book from friends
    started_at    timestamptz NOT NULL DEFAULT now(),
    last_read_at  timestamptz NOT NULL DEFAULT now(),
    finished_at   timestamptz,
    PRIMARY KEY (user_id, edition_id)
);

-- One row per page per user per local day. Doubles as the streak source of truth and
-- the metering ledger: re-reading the same page on the same day is free.
CREATE TABLE page_reads (
    user_id   uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    page_id   uuid NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    local_day date NOT NULL,                    -- computed in the user's timezone
    metered   boolean NOT NULL,                 -- true only for licensed content
    read_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, page_id, local_day)
);
CREATE INDEX page_reads_quota  ON page_reads (user_id, local_day) WHERE metered;
CREATE INDEX page_reads_streak ON page_reads (user_id, local_day);

CREATE TABLE streaks (
    user_id      uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    current_days int  NOT NULL DEFAULT 0,
    longest_days int  NOT NULL DEFAULT 0,
    last_day     date,
    freezes_left int  NOT NULL DEFAULT 0        -- Plus perk, UI is v2
);

-- ----------------------------------------------------------------- social

CREATE TABLE page_likes (
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    page_id    uuid NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, page_id)
);
CREATE INDEX ON page_likes (page_id);

CREATE TABLE highlights (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    page_id    uuid NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    start_off  int  NOT NULL,                   -- character offsets into pages.content
    end_off    int  NOT NULL,
    quote      text NOT NULL,                   -- denormalised so it survives a takedown
    share_slug text UNIQUE,                     -- set when shared publicly
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (end_off > start_off)
);
CREATE INDEX ON highlights (user_id, created_at DESC);
CREATE INDEX ON highlights (page_id);

CREATE TYPE friendship_state AS ENUM ('pending', 'accepted', 'blocked');

-- Mutual-accept friendship.
--
-- requester_id/addressee_id record the TRUE direction, so the UI can say "they asked
-- you" rather than the reverse. Uniqueness is on the *unordered* pair via the
-- expression index below, which is what stops A->B and B->A both existing.
CREATE TABLE friendships (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state        friendship_state NOT NULL DEFAULT 'pending',
    created_at   timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz,
    CHECK (requester_id <> addressee_id)
);

-- One relationship per pair of people, in whichever direction it was created.
CREATE UNIQUE INDEX friendships_unique_pair ON friendships
    (LEAST(requester_id, addressee_id), GREATEST(requester_id, addressee_id));

CREATE INDEX ON friendships (addressee_id, state);
CREATE INDEX ON friendships (requester_id, state);

-- ----------------------------------------------------------------- trust & safety

CREATE TABLE takedown_notices (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    edition_id     uuid REFERENCES book_editions(id) ON DELETE SET NULL,
    reporter_email citext NOT NULL,
    claim          text NOT NULL,
    status         text NOT NULL DEFAULT 'open',
    created_at     timestamptz NOT NULL DEFAULT now(),
    resolved_at    timestamptz
);
