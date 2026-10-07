package main

import (
	"context"
	"fmt"
	"log"

	"github.com/bookworm/api/internal/ingest"
)

// The fake commercial catalogue.
//
// These are invented publishers and invented books, written for this repository.
// Nothing here is a real work and nothing is under anyone else's copyright --
// which is the point: the paid tier needs content that is genuinely NOT public
// domain in order to exercise the meter, and borrowing a real licensed book to
// do that would be the exact thing the product is built to avoid.
//
// They also exist to make the publisher pipeline real: each one is attached to a
// publisher row with its own verification state, so the catalogue exercises the
// same path a genuine upload would take.

type fakePublisher struct {
	Key      string
	Name     string
	Email    string
	Handle   string
	Verified bool // unverified publishers model the pre-review state
}

var fakePublishers = []fakePublisher{
	{"northwind", "Northwind Press", "rights@northwind.example", "northwind_press", true},
	{"meridian", "Meridian Books", "rights@meridian.example", "meridian_books", true},
	// Deliberately unverified: this is what an upload looks like before a human
	// has checked the uploader actually holds the rights. See docs/LEGAL.md.
	{"brightfolio", "Brightfolio", "hello@brightfolio.example", "brightfolio", false},
}

type commercialBook struct {
	Publisher string
	File      string
	Slug      string
	Title     string
	Author    string
	Blurb     string
}

var commercialCatalogue = []commercialBook{
	{
		Publisher: "northwind",
		File:      "seed/commercial/northwind-the-salt-road.txt",
		Slug:      "the-salt-road",
		Title:     "The Salt Road",
		Author:    "Eilish Mara Brennan",
		Blurb:     "The lamp fails at 3:14 in the morning, there is a boat somewhere past the point, and the keeper has been sitting in the dark for three hours trying to want it to go down.",
	},
	{
		Publisher: "northwind",
		File:      "seed/commercial/northwind-the-glasshouse-winter.txt",
		Slug:      "the-glasshouse-winter",
		Title:     "The Glasshouse Winter",
		Author:    "Tomas Reidar Lindqvist",
		Blurb:     "A structural engineer is hired to assess a dying Victorian glasshouse, and discovers he has really been hired to say the thing nobody else will.",
	},
	{
		Publisher: "meridian",
		File:      "seed/commercial/meridian-quiet-machines.txt",
		Slug:      "quiet-machines",
		Title:     "Quiet Machines",
		Author:    "Dr Anneke Vos",
		Blurb:     "A 1948 school thermostat, and the forty ways we have reinvented it since. On measurement, proxies, and the slow theft of the questions we meant to ask.",
	},
	{
		Publisher: "meridian",
		File:      "seed/commercial/meridian-the-borrowed-hour.txt",
		Slug:      "the-borrowed-hour",
		Title:     "The Borrowed Hour",
		Author:    "Jonathan Ifeanyi Clarke",
		Blurb:     "Bristol kept two kinds of noon for forty-eight years. A history of coordination, and why your calendar is still arguing about it.",
	},
	{
		Publisher: "brightfolio",
		File:      "seed/commercial/brightfolio-the-last-exhibit.txt",
		Slug:      "the-last-exhibit",
		Title:     "The Last Exhibit",
		Author:    "Priya Raghunathan",
		Blurb:     "A nine-year-old notices the shadow falls the wrong way. By nine that evening the curator has three facts that refuse to make one story.",
	},
	{
		Publisher: "brightfolio",
		File:      "seed/commercial/brightfolio-sixteen-winters.txt",
		Slug:      "sixteen-winters",
		Title:     "Sixteen Winters",
		Author:    "Hanna Kowalczyk-Reilly",
		Blurb:     "Three hundred and eleven names in a school exercise book, and the one debt from 1951 that could never be settled.",
	},
}

// seedCommercial creates the fake publishers and publishes their catalogue as
// licensed content.
func seedCommercial(ctx context.Context, ing *ingest.Ingester, pubIDs map[string]string) error {
	for _, cb := range commercialCatalogue {
		pubID, ok := pubIDs[cb.Publisher]
		if !ok {
			log.Printf("skipping %s: no publisher %q", cb.Title, cb.Publisher)
			continue
		}

		book, err := loadFile(cb.File, cb.Slug, cb.Title, cb.Author, cb.Blurb, "licensed")
		if err != nil {
			log.Printf("skipping %s: %v", cb.Title, err)
			continue
		}
		book.PublisherID = pubID
		book.SourceRef = "publisher:" + cb.Publisher

		res, err := ing.Publish(ctx, *book)
		if err != nil {
			log.Printf("failed to publish %s: %v", cb.Title, err)
			continue
		}

		verb := "ingested"
		if !res.Published {
			verb = "unchanged"
		}
		fmt.Printf("%-9s  %-24s %-18s %4d pages\n", verb, cb.Title, cb.Publisher, res.PageCount)
	}
	return nil
}
