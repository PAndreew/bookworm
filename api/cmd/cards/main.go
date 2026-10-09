// cards prepares the non-destructive presentation layer for published editions.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bookworm/api/internal/config"
	"github.com/bookworm/api/internal/db"
	"github.com/bookworm/api/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	slug := flag.String("slug", "", "prepare only this book (default: all published editions)")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool); err != nil {
		return err
	}
	rows, err := pool.Query(ctx, `SELECT e.id FROM book_editions e JOIN books b ON b.id=e.book_id WHERE e.state='published' AND ($1='' OR b.slug=$1) ORDER BY e.id`, *slug)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if *slug != "" && len(ids) == 0 {
		return fmt.Errorf("no published book with slug %q", *slug)
	}
	st := store.New(pool)
	for _, id := range ids {
		if err = st.EnsureReaderCards(ctx, id); err != nil {
			return err
		}
	}
	fmt.Printf("Prepared balanced card layouts for %d published editions; obsolete layouts discarded, original anchors/history preserved.\n", len(ids))
	return nil
}
