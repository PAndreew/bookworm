// Thin client for the Go API.
//
// Every call sends cookies: the session is an HttpOnly cookie, so there is no token
// for JavaScript to hold (or leak).

export type Tier = 'free' | 'plus' | 'pro';

export interface User {
  id: string;
  email: string;
  handle: string;
  display_name: string;
  timezone: string;
  role: string;
  tier: Tier;
  visibility: 'public' | 'friends' | 'private';
  totp_enabled: boolean;
}

export interface Book {
  id: string;
  slug: string;
  title: string;
  author: string;
  description: string;
  cover_url: string | null;
  license: 'public_domain' | 'licensed';
  edition_id: string;
  page_count: number;
  card_count: number;
  word_count: number;
  genres: string[];
}

export interface Page {
  card_id?: string;
  kind?: 'text' | 'title' | 'chapter';
  source_idx?: number;
  start_off?: number;
  id: string;
  idx: number;
  chapter_title: string | null;
  content: string;
  char_count: number;
  like_count: number;
  liked_by_me: boolean;
}

export interface ReadResult {
  metered: boolean;
  remaining: number;
  pages_today: number;
  streak_days: number;
  streak_extended: boolean;
  freeze_used: boolean;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public body: any = null,
  ) {
    super(message);
  }

  /** True when the caller hit a tier limit rather than made a mistake. */
  get isPaywall() {
    return this.status === 402;
  }
}

const BASE = '/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(BASE + path, {
    ...init,
    credentials: 'include',
    headers: {
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const body = text ? JSON.parse(text) : null;

  if (!res.ok) {
    throw new ApiError(
      res.status,
      body?.error ?? 'unknown',
      body?.message ?? 'Something went wrong.',
      body,
    );
  }
  return body as T;
}

const post = <T>(path: string, data?: unknown) =>
  request<T>(path, { method: 'POST', body: data ? JSON.stringify(data) : undefined });

export const api = {
  // ---------------------------------------------------------------- auth
  signup: (data: {
    email: string;
    handle: string;
    display_name?: string;
    password: string;
    birthdate: string;
    timezone: string;
  }) => post<{ user: User }>('/auth/signup', data),

  login: (email: string, password: string) =>
    post<{ user?: User; mfa_required?: boolean }>('/auth/login', { email, password }),

  verify2FA: (code: string) => post<{ user: User }>('/auth/2fa/verify', { code }),
  verifyRecovery: (recovery_code: string) =>
    post<{ user: User }>('/auth/2fa/verify', { recovery_code }),

  logout: () => post<{ status: string }>('/auth/logout'),

  setup2FA: () => post<{ provisioning_uri: string }>('/auth/2fa/setup'),
  enable2FA: (code: string) => post<{ recovery_codes: string[] }>('/auth/2fa/enable', { code }),
  disable2FA: (password: string) => post<{ status: string }>('/auth/2fa/disable', { password }),

  // ------------------------------------------------------------------ me
  me: () =>
    request<{
      user: User;
      last_read: { slug: string; title: string; page_idx: number; card_idx: number | null } | null;
      quota: { limit: number; used: number; resets_at: string; period: 'day' | 'month' };
      streak: { current: number; longest: number; at_risk: boolean };
    }>('/me'),

  updateMe: (data: { display_name?: string; visibility?: string; timezone?: string }) =>
    request<{ user: User }>('/me', { method: 'PATCH', body: JSON.stringify(data) }),

  // ------------------------------------------------------------- catalog
  books: (options: { license?: string; q?: string; genre?: string; offset?: number } = {}, signal?: AbortSignal) => {
    const query = new URLSearchParams({ limit: '24', offset: String(options.offset ?? 0) });
    if (options.license) query.set('license', options.license);
    if (options.q) query.set('q', options.q);
    if (options.genre) query.set('genre', options.genre);
    return request<{ books: Book[]; next_offset: number | null }>(`/books?${query}`, { signal });
  },

  genres: () => request<{ genres: string[] }>('/genres'),

  ratings: () => request<{ ratings: Record<string, number> }>('/me/ratings'),
  rateBook: (slug: string, stars: number) =>
    request<{ stars: number }>(`/books/${encodeURIComponent(slug)}/rating`, {
      method: 'PUT', body: JSON.stringify({ stars }),
    }),

  book: (slug: string) => request<{ book: Book }>(`/books/${slug}`),

  pages: (slug: string, from = 0, limit = 10) =>
    request<{ pages: Page[]; page_count: number }>(
      `/books/${slug}/pages?from=${from}&limit=${limit}`,
    ),

  assistant: (data: { mode: 'explain' | 'translate'; phrase: string; language: string; consent: boolean }, signal?: AbortSignal) =>
    request<{ answer: string; model: string }>('/assistant', { method: 'POST', body: JSON.stringify(data), signal }),

  // ------------------------------------------------------------- reading
  cards: (slug: string, from = 0, limit = 12, sourceFrom = 0) =>
    request<{ pages: Page[]; page_count: number }>(`/books/${encodeURIComponent(slug)}/cards?card_from=${from}&source_from=${sourceFrom}&limit=${limit}`),
  recordCardRead: (card_id: string) => post<ReadResult>('/card-reads', { card_id }),
  recordRead: (page_id: string) => post<ReadResult>('/reads', { page_id }),

  like: (pageId: string) => post<{ liked: boolean }>(`/pages/${pageId}/like`),
  unlike: (pageId: string) =>
    request<{ liked: boolean }>(`/pages/${pageId}/like`, { method: 'DELETE' }),

  createHighlight: (data: {
    page_id: string;
    start_off: number;
    end_off: number;
    quote: string;
    share: boolean;
  }) => post<{ highlight: any }>('/highlights', data),

  highlights: () => request<{ highlights: any[] }>('/highlights'),
  deleteHighlight: (id: string) => request<void>(`/highlights/${id}`, { method: 'DELETE' }),

  // -------------------------------------------------------------- social
  streak: () =>
    request<{
      current: number;
      longest: number;
      at_risk: boolean;
      pages_today: number;
      pages_required: number;
      freezes_left: number;
    }>('/streak'),

  friends: () => request<{ friends: any[] }>('/friends'),
  pendingFriends: () => request<{ pending: any[] }>('/friends/pending'),
  requestFriend: (handle: string) => post<{ status: string }>(`/friends/${handle}`),
  acceptFriend: (handle: string) => post<{ status: string }>(`/friends/${handle}/accept`),
  removeFriend: (handle: string) =>
    request<void>(`/friends/${handle}`, { method: 'DELETE' }),
  blockUser: (handle: string) => post<{ status: string }>(`/friends/${handle}/block`),

  searchUsers: (q: string) => request<{ users: any[] }>(`/users/search?q=${encodeURIComponent(q)}`),
  profile: (handle: string) => request<{ profile: any }>(`/users/${handle}`),
  friendFeed: () => request<{ activity: any[] }>('/feed/friends'),
};
