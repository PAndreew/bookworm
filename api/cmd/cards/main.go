// cards prepares the non-destructive presentation layer for published editions.
package main

import (
	"context"
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
	rows, err := pool.Query(ctx, `SELECT id FROM book_editions WHERE state='published' ORDER BY id`)
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
	st := store.New(pool)
	for _, id := range ids {
		if err = st.EnsureReaderCards(ctx, id); err != nil {
			return err
		}
	}
	fmt.Printf("Prepared card layouts for %d published editions; original pages untouched.\n", len(ids))
	return nil
}
