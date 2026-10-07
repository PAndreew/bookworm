// Command ingest loads books into the catalogue.
//
// Public-domain titles are fetched from Project Gutenberg, which distributes works
// whose copyright has expired. The Gutenberg licence header and footer are stripped
// before chunking so no boilerplate ends up in the feed.
//
//	go run ./cmd/ingest -seed            # the curated starter shelf
//	go run ./cmd/ingest -gutenberg 1342  # one book by Gutenberg id
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bookworm/api/internal/auth"
	"github.com/bookworm/api/internal/chunk"
	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/ingest"
)

func main() {
	var (
		seed       = flag.Bool("seed", false, "ingest the curated starter shelf (needs internet)")
		seedLocal  = flag.Bool("seed-local", false, "ingest the offline sample texts in ./seed")
		gutenberg  = flag.Int("gutenberg", 0, "ingest one Project Gutenberg book by id")
		demo       = flag.Bool("demo-licensed", false, "add the seeded demo licensed titles")
		commercial = flag.Bool("commercial", false, "add the fake publishers and their licensed catalogue")
		all        = flag.Bool("all", false, "full local catalogue: public-domain shelf + fake commercial titles")
		replace    = flag.Bool("replace", false, "with -file: replace the pages of an existing edition")

		file    = flag.String("file", "", "ingest a local plain-text file")
		slug    = flag.String("slug", "", "slug for -file")
		title   = flag.String("title", "", "title for -file")
		author  = flag.String("author", "", "author for -file")
		blurb   = flag.String("description", "", "description for -file")
		license = flag.String("license", "public_domain", "public_domain | licensed")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatal(err)
	}

	ing := ingest.New(pool)

	switch {
	case *all:
		if err := seedShelf(ctx, ing); err != nil {
			log.Fatal(err)
		}
		pubIDs, err := ensurePublishers(ctx, ing)
		if err != nil {
			log.Fatal(err)
		}
		if err := seedCommercial(ctx, ing, pubIDs); err != nil {
			log.Fatal(err)
		}

	case *commercial:
		pubIDs, err := ensurePublishers(ctx, ing)
		if err != nil {
			log.Fatal(err)
		}
		if err := seedCommercial(ctx, ing, pubIDs); err != nil {
			log.Fatal(err)
		}

	case *file != "":
		book, err := loadFile(*file, *slug, *title, *author, *blurb, *license)
		if err != nil {
			log.Fatal(err)
		}
		// -replace swaps the text of an existing edition; see ingest.Replace for
		// why that is deliberately hard to do by accident.
		pub := ing.Publish
		if *replace {
			pub = ing.Replace
		}
		res, err := pub(ctx, *book)
		if err != nil {
			log.Fatal(err)
		}
		if res.Published {
			fmt.Printf("ingested %s by %s (%d pages)\n", book.Title, book.Author, res.PageCount)
		} else {
			fmt.Printf("unchanged: %s already has an edition at chunker v%d (%d pages). "+
				"Use -replace to swap the text.\n", book.Slug, chunk.Version, res.PageCount)
		}

	case *seedLocal:
		if err := seedOffline(ctx, ing); err != nil {
			log.Fatal(err)
		}

	case *gutenberg > 0:
		book, err := fetchGutenberg(ctx, *gutenberg)
		if err != nil {
			log.Fatal(err)
		}
		res, err := ing.Publish(ctx, *book)
		if err != nil {
			log.Fatal(err)
		}
		verb := "ingested"
		if !res.Published {
			verb = "unchanged"
		}
		fmt.Printf("%s %s by %s (%d pages)\n", verb, book.Title, book.Author, res.PageCount)

	case *seed:
		if err := seedShelf(ctx, ing); err != nil {
			log.Fatal(err)
		}

	case *demo:
		if err := seedLicensedDemo(ctx, ing); err != nil {
			log.Fatal(err)
		}

	default:
		flag.Usage()
		os.Exit(2)
	}
}

// starterShelf is the curated public-domain launch set: recognisable titles that
// read well in short cards.
var starterShelf = []struct {
	ID     int
	Slug   string
	Title  string
	Author string
	Blurb  string
}{
	{1342, "pride-and-prejudice", "Pride and Prejudice", "Jane Austen",
		"Elizabeth Bennet, five sisters, and the slow collapse of a very good first impression."},
	{84, "frankenstein", "Frankenstein", "Mary Shelley",
		"A student builds a living thing and cannot bear to look at what he made."},
	{1661, "the-adventures-of-sherlock-holmes", "The Adventures of Sherlock Holmes", "Arthur Conan Doyle",
		"Twelve cases, one impossible man, and a narrator who never quite keeps up."},
	{2701, "moby-dick", "Moby-Dick", "Herman Melville",
		"One captain's obsession, told by the only man who lived to tell it."},
	{345, "dracula", "Dracula", "Bram Stoker",
		"Letters and diaries from people who realise, too late, what is in the house."},
	{11, "alices-adventures-in-wonderland", "Alice's Adventures in Wonderland", "Lewis Carroll",
		"A girl falls down a hole and finds a world that argues back."},
	{174, "the-picture-of-dorian-gray", "The Picture of Dorian Gray", "Oscar Wilde",
		"A portrait ages. Its subject does not. Neither improves."},
	{1952, "the-yellow-wallpaper", "The Yellow Wallpaper", "Charlotte Perkins Gilman",
		"A woman is told to rest. She studies the wallpaper instead."},
	{64317, "the-great-gatsby", "The Great Gatsby", "F. Scott Fitzgerald",
		"A man throws parties for one guest who never comes."},
	{98, "a-tale-of-two-cities", "A Tale of Two Cities", "Charles Dickens",
		"London and Paris, a revolution, and a man with nothing to lose."},
}

func seedShelf(ctx context.Context, ing *ingest.Ingester) error {
	for _, s := range starterShelf {
		book, err := fetchGutenberg(ctx, s.ID)
		if err != nil {
			log.Printf("skipping %s: %v", s.Title, err)
			continue
		}
		// Prefer our curated metadata over whatever the plain text header says.
		book.Slug = s.Slug
		book.Title = s.Title
		book.Author = s.Author
		book.Description = s.Blurb

		res, err := ing.Publish(ctx, *book)
		if err != nil {
			log.Printf("failed to publish %s: %v", s.Title, err)
			continue
		}
		verb := "ingested"
		if !res.Published {
			verb = "unchanged"
		}
		fmt.Printf("%-9s  %-40s %5d pages\n", verb, s.Title, res.PageCount)
	}
	return nil
}

// seedLicensedDemo adds placeholder "licensed" titles so the Plus and Pro tiers are
// demonstrable before any real licensing deal exists. The text is public domain --
// only the licence flag is set to 'licensed', purely to exercise the meter.
func seedLicensedDemo(ctx context.Context, ing *ingest.Ingester) error {
	demos := []struct {
		ID     int
		Slug   string
		Title  string
		Author string
	}{
		{43, "the-strange-case-of-dr-jekyll-and-mr-hyde", "The Strange Case of Dr Jekyll and Mr Hyde", "Robert Louis Stevenson"},
		{1400, "great-expectations", "Great Expectations", "Charles Dickens"},
	}

	for _, d := range demos {
		book, err := fetchGutenberg(ctx, d.ID)
		if err != nil {
			log.Printf("skipping %s: %v", d.Title, err)
			continue
		}
		book.Slug = "demo-" + d.Slug
		book.Title = d.Title + " (demo licensed)"
		book.Author = d.Author
		book.Description = "A demo title flagged as licensed, to exercise the Plus and Pro page meter."
		book.License = "licensed"

		res, err := ing.Publish(ctx, *book)
		if err != nil {
			log.Printf("failed to publish %s: %v", d.Title, err)
			continue
		}
		fmt.Printf("licensed  %-30s %5d pages\n", d.Title, res.PageCount)
	}
	return nil
}

// fetchGutenberg downloads a book's plain text and chunks it into pages.
func fetchGutenberg(ctx context.Context, id int) (*ingest.Book, error) {
	meta, err := fetchGutendexMeta(ctx, id)
	if err != nil {
		// Metadata is a nicety; the text is what matters.
		log.Printf("no metadata for %d: %v", id, err)
		meta = &gutendexBook{}
	}

	text, err := fetchGutenbergText(ctx, id)
	if err != nil {
		return nil, err
	}

	cleaned := chunk.StripGutenbergBoilerplate(text)
	pages := chunk.Split(cleaned)
	if len(pages) == 0 {
		return nil, fmt.Errorf("ingest: book %d produced no pages", id)
	}

	author := "Unknown"
	if len(meta.Authors) > 0 {
		author = meta.Authors[0].Name
	}

	return &ingest.Book{
		Slug:      fmt.Sprintf("gutenberg-%d", id),
		Title:     orDefault(meta.Title, fmt.Sprintf("Gutenberg %d", id)),
		Author:    author,
		Language:  "en",
		License:   "public_domain",
		SourceRef: fmt.Sprintf("gutenberg:%d", id),
		Pages:     pages,
		WordCount: len(strings.Fields(cleaned)),
	}, nil
}

type gutendexBook struct {
	Title   string `json:"title"`
	Authors []struct {
		Name string `json:"name"`
	} `json:"authors"`
}

func fetchGutendexMeta(ctx context.Context, id int) (*gutendexBook, error) {
	url := fmt.Sprintf("https://gutendex.com/books/%d", id)
	body, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	var b gutendexBook
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// fetchGutenbergText tries the usual plain-text URL shapes; Gutenberg is not
// consistent about which one a given book uses.
func fetchGutenbergText(ctx context.Context, id int) (string, error) {
	urls := []string{
		fmt.Sprintf("https://www.gutenberg.org/cache/epub/%d/pg%d.txt", id, id),
		fmt.Sprintf("https://www.gutenberg.org/files/%d/%d-0.txt", id, id),
		fmt.Sprintf("https://www.gutenberg.org/files/%d/%d.txt", id, id),
	}
	var lastErr error
	for _, u := range urls {
		body, err := get(ctx, u)
		if err == nil && len(body) > 1000 {
			return string(body), nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("ingest: could not fetch text for %d: %w", id, lastErr)
}

func get(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Gutenberg blocks unidentified bulk clients, reasonably enough.
	req.Header.Set("User-Agent", "Bookworm/1.0 (public-domain ingest)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s returned %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20)) // 20 MiB ceiling
}

func orDefault(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// loadFile ingests a plain-text file from disk. This is how the offline samples
// are loaded, and how a publisher upload will be tested before the portal exists.
func loadFile(path, slug, title, author, blurb, license string) (*ingest.Book, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cleaned := chunk.StripGutenbergBoilerplate(string(raw))
	pages := chunk.Split(cleaned)
	if len(pages) == 0 {
		return nil, fmt.Errorf("ingest: %s produced no pages", path)
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if slug == "" {
		slug = base
	}
	if title == "" {
		title = base
	}
	if author == "" {
		author = "Unknown"
	}

	return &ingest.Book{
		Slug:        slug,
		Title:       title,
		Author:      author,
		Description: blurb,
		Language:    "en",
		License:     license,
		SourceRef:   "file:" + filepath.Base(path),
		Pages:       pages,
		WordCount:   len(strings.Fields(cleaned)),
	}, nil
}

// offlineSeed describes the sample texts shipped in ./seed so the app can be run
// and demonstrated with no network access.
var offlineSeed = []struct {
	File    string
	Slug    string
	Title   string
	Author  string
	Blurb   string
	License string
}{
	{"seed/alices-adventures-in-wonderland.txt", "alices-adventures-in-wonderland",
		"Alice's Adventures in Wonderland", "Lewis Carroll",
		"A girl falls down a hole and finds a world that argues back.", "public_domain"},
	{"seed/the-yellow-wallpaper.txt", "the-yellow-wallpaper",
		"The Yellow Wallpaper", "Charlotte Perkins Gilman",
		"A woman is told to rest. She studies the wallpaper instead.", "public_domain"},
	// Flagged licensed purely to exercise the Plus/Pro meter. The text itself is
	// public domain -- there is no real licensed catalogue yet.
	{"seed/the-yellow-wallpaper.txt", "demo-licensed-sample",
		"A Demo Licensed Title", "Charlotte Perkins Gilman",
		"A demo title flagged as licensed, to exercise the Plus and Pro page meter.", "licensed"},
}

func seedOffline(ctx context.Context, ing *ingest.Ingester) error {
	for _, s := range offlineSeed {
		book, err := loadFile(s.File, s.Slug, s.Title, s.Author, s.Blurb, s.License)
		if err != nil {
			log.Printf("skipping %s: %v", s.Title, err)
			continue
		}
		res, err := ing.Publish(ctx, *book)
		if err != nil {
			log.Printf("failed to publish %s: %v", s.Title, err)
			continue
		}
		verb := "ingested"
		if !res.Published {
			verb = "unchanged"
		}
		fmt.Printf("%-9s  %-34s %-14s %4d pages\n", verb, s.Title, s.License, res.PageCount)
	}
	return nil
}

// ensurePublishers creates the fake publisher accounts and returns their ids.
//
// All of them share one throwaway password. These are demo accounts in a local
// database; the hash is generated through the real code path so the rows are
// shaped exactly like a genuine signup, but nobody should ever sign in as one.
func ensurePublishers(ctx context.Context, ing *ingest.Ingester) (map[string]string, error) {
	hash, err := auth.HashPassword("demo-publisher-account-not-for-use")
	if err != nil {
		return nil, err
	}

	ids := make(map[string]string, len(fakePublishers))
	for _, fp := range fakePublishers {
		p, err := ing.EnsurePublisher(ctx, fp.Name, fp.Email, fp.Handle, hash, fp.Verified)
		if err != nil {
			return nil, fmt.Errorf("publisher %s: %w", fp.Name, err)
		}
		ids[fp.Key] = p.ID

		status := "verified"
		if !p.Verified {
			status = "UNVERIFIED (pending review)"
		}
		fmt.Printf("publisher  %-20s %s\n", p.Name, status)
	}
	return ids, nil
}
