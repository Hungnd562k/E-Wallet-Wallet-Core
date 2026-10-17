// Command migrate applies the embedded SQL migrations.
//
//	go run ./cmd/migrate up        # apply all pending migrations
//	go run ./cmd/migrate down 1    # roll back the last N migrations
//	go run ./cmd/migrate version   # print the current version
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"

	"github.com/Hungnd562k/E-Wallet-Wallet-Core/migrations"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: migrate up | down N | version")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}

	m, err := migrations.New(url)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }() // returns source and database close errors; nothing useful to do with them

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		if len(args) < 2 {
			return errors.New("down needs a step count, e.g. down 1")
		}
		n, convErr := strconv.Atoi(args[1])
		if convErr != nil || n < 1 {
			return fmt.Errorf("invalid step count %q", args[1])
		}
		err = m.Steps(-n)
	case "version":
		v, dirty, vErr := m.Version()
		if errors.Is(vErr, migrate.ErrNilVersion) {
			fmt.Println("version=none (no migration applied)")
			return nil
		}
		if vErr != nil {
			return vErr
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no change")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Println("done")
	return nil
}
