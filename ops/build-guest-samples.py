#!/usr/bin/env python3
"""Freeze ten public-domain reader cards per guest book; never call the AI API."""
import json
from pathlib import Path
from urllib.request import urlopen

BOOKS = [
    ('gutenberg-46', 'A Christmas Carol', 'Charles Dickens', '#edf4e8', [
        ('endeavoured', 'Tried hard to do or achieve something.'),
        ('faithful', 'Loyal, dependable, and true to someone.'),
        ('clergyman', 'A man who is a religious minister, especially in a Christian church.'),
        ('simile', 'A comparison using “like” or “as”—here, “as dead as a door-nail.”'),
        ('executor', 'The person responsible for carrying out the instructions in someone’s will after they die.'),
        ('solemnised', 'Marked an occasion with a formal ceremony or observance. Dickens uses it ironically about Scrooge making a bargain.'),
        ('ramparts', 'The defensive walls or raised banks around a castle or fort.'),
        ('astonish', 'To surprise someone greatly.'),
        ('covetous', 'Having a strong desire to possess things, especially things belonging to other people.'),
    ]),
    ('gutenberg-4363', 'Beyond Good and Evil', 'Friedrich Nietzsche', '#f3eadf', [
        ('adaptations', 'Changes made to something so it suits a different use or format.'),
        ('italicized', 'Printed in slanted letters, often to emphasize a word or phrase.'),
        ('prejudices', 'Opinions formed in advance, without fairly considering the evidence.'),
        ('apophthegms', 'Short, memorable sayings expressing a general truth or piece of wisdom.'),
        ('importunity', 'Persistent, troublesome insistence or demands.'),
        ('mien', 'A person’s appearance or manner, especially as an expression of their mood.'),
        ('tyronism', 'The inexperience or immaturity of a beginner.'),
        ('audacious', 'Bold and daring, sometimes in a way that disregards accepted limits.'),
        ('acuteness', 'Sharpness of understanding or perception.'),
    ]),
    ('the-yellow-wallpaper', 'The Yellow Wallpaper', 'Charlotte Perkins Gilman', '#f7f0c9', [
        ('felicity', 'Great happiness or bliss.'),
        ('untenanted', 'Not occupied by a tenant; empty.'),
        ('physician', 'A medical doctor.'),
        ('tonics', 'Medicines or preparations believed to improve strength or general health.'),
        ('congenial', 'Pleasant and well suited to a person’s tastes or needs.'),
        ('stimulus', 'Something that encourages activity, interest, or a response.'),
        ('hedges', 'Rows of closely planted bushes forming a boundary or screen.'),
        ('arbors', 'Shaded garden structures or passages covered with climbing plants.'),
        ('draught', 'A current of cool air entering a room; the British spelling of “draft.”'),
    ]),
    ('gutenberg-86', 'A Connecticut Yankee in King Arthur’s Court', 'Mark Twain', '#e8eef5', [
        ('ungentle', 'Harsh, unkind, or lacking gentleness.'),
        ('libel', 'A written statement that unfairly harms someone’s reputation; here, Twain jokingly applies it to an entire century.'),
        ('indisputable', 'So clearly true that it cannot reasonably be challenged.'),
        ('deduction', 'A conclusion reached by reasoning from other statements or facts.'),
        ('scheme', 'An organized plan or system; here, the author’s line of argument.'),
        ('candid', 'Honest, open, and straightforward.'),
        ('imperceptibly', 'So gradually or subtly that it is difficult to notice.'),
        ('antiquity', 'The distant past, especially ancient times.'),
        ('musty', 'Having the stale smell or character of something old and long unused.'),
    ]),
]

if __name__ == '__main__':
    import re
    samples = []
    for slug, title, author, color, glossary in BOOKS:
        with urlopen(f'https://potpuri.cc/v1/books/{slug}/cards?card_from=0&limit=10') as response:
            cards = json.load(response)['pages']
        assert len(cards) == 10, slug
        pages = []
        for i, card in enumerate(cards):
            word, definition = glossary[i - 1] if i else ('', '')
            assert not word or re.search(r'\b' + re.escape(word) + r'\b', card['content'], re.I), (slug, i, word)
            pages.append(dict(content=card['content'], kind=card.get('kind', 'text'), word=word, definition=definition))
        samples.append(dict(slug=slug, title=title, author=author, color=color, license='public_domain', pages=pages))
    target = Path(__file__).resolve().parents[1] / 'web/src/lib/guest-books.json'
    target.write_text(json.dumps(samples, ensure_ascii=False, indent=2) + '\n')
    print(f'Wrote {len(samples)} books / {sum(len(b["pages"]) for b in samples)} cards to {target}')
