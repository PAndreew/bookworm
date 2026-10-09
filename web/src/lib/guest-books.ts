import books from './guest-books.json';
import preparedTranslations from './guest-translations.json';
export function guestTranslation(book: GuestBook | undefined, idx: number): string {
  if (!book || idx !== 2) return '';
  const sample = preparedTranslations[book.slug as keyof typeof preparedTranslations];
  return sample?.source === book.pages[idx]?.content ? sample.content : '';
}
import type { Page } from './api';
export { books };
export type GuestBook = (typeof books)[number];
export function guestPages(book: GuestBook): Page[] {
  return book.pages.slice(0, 10).map((card, idx) => ({
    id: `demo-source-${book.slug}-${idx}`, card_id: `demo-${book.slug}-${idx}`, idx,
    kind: card.kind === 'title' ? 'title' : card.kind === 'chapter' ? 'chapter' : 'text',
    content: card.content, sentence_ends: card.sentence_ends, chapter_title: null, char_count: Array.from(card.content).length,
    like_count: 0, liked_by_me: false,
  }));
}
