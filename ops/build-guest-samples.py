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

TITLE_WORDS = {
    'gutenberg-46': ('Carol', 'A song of joy or celebration, especially one sung at Christmas.'),
    'gutenberg-4363': ('Beyond', 'Outside or further than a boundary or limit; here, outside conventional ideas of good and evil.'),
    'the-yellow-wallpaper': ('Wallpaper', 'Decorative paper used to cover the walls of a room.'),
    'gutenberg-86': ('Yankee', 'A person from the northern United States, especially New England, where Connecticut is located.'),
}

if __name__ == '__main__':
    import re
    import os
    import subprocess
    root = Path(__file__).resolve().parents[1]
    samples = []
    for slug, title, author, color, glossary in BOOKS:
        with urlopen(f'https://potpuri.cc/v1/books/{slug}/pages?from=0&limit=20') as response:
            source = json.load(response)
        # The demo already has a dedicated title card. Omit repeated imprint
        # headings from its first prose card, not any of the prose itself.
        opening = {
            'gutenberg-46': 'I HAVE endeavoured',
            'gutenberg-4363': 'The following is a reprint',
            'the-yellow-wallpaper': 'It is very seldom',
            'gutenberg-86': 'The ungentle laws',
        }[slug]
        first = source['pages'][0]['content']
        assert opening in first, (slug, 'opening changed')
        source['pages'][0]['content'] = first[first.index(opening):]
        result = subprocess.run([os.environ.get('GO', 'go'), 'run', './cmd/card-sample'],
            input=json.dumps(source).encode(), capture_output=True, check=True, cwd=root / 'api')
        chunks = json.loads(result.stdout)
        assert len(chunks) >= 9, slug
        cards = [dict(kind='title', content=f'{title}\n{author}', sentence_ends=[])] + [dict(kind='text', **s) for s in chunks[:9]]
        pages = []
        for i, card in enumerate(cards):
            word, definition = next(((w, d) for w, d in glossary
                if re.search(r'\b' + re.escape(w) + r'\b', card['content'], re.I)), ('', '')) if i else TITLE_WORDS[slug]
            pages.append(dict(content=card['content'], sentence_ends=card['sentence_ends'], kind=card.get('kind', 'text'), word=word, definition=definition))
        with urlopen(f'https://potpuri.cc/v1/books/{slug}') as response:
            metadata = json.load(response)['book']
        from urllib.parse import quote
        with urlopen(f'https://potpuri.cc/v1/books?q={quote(metadata["title"])}&limit=100') as response:
            catalog = json.load(response)['books']
        reviewed = next(b for b in catalog if b['slug'] == slug)
        samples.append(dict(slug=slug, title=metadata['title'], author=metadata['author'],
            license='public_domain', page_count=metadata['page_count'], card_count=metadata['card_count'],
            genres=metadata['genres'], rating_average=reviewed.get('rating_average', 0),
            rating_count=reviewed.get('rating_count', 0), pages=pages))
    target = Path(__file__).resolve().parents[1] / 'web/src/lib/guest-books.json'
    target.write_text(json.dumps(samples, ensure_ascii=False, indent=2) + '\n')
    print(f'Wrote {len(samples)} books / {sum(len(b["pages"]) for b in samples)} cards to {target}')
