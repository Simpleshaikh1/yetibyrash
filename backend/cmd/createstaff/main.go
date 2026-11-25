// Command createstaff adds a staff account from the command line.
// Use it once on a new server to create the first owner; after that, owners add staff in the admin.
//
//	go run ./cmd/createstaff -email you@example.com -name "Your Name" -role owner
//
// It asks for the password (typed on the next line), so it never appears in your shell history.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"yetibyrash/backend/internal/config"
	"yetibyrash/backend/internal/db"
	"yetibyrash/backend/internal/staff"
)

func main() {
	email := flag.String("email", "", "staff email (required)")
	name := flag.String("name", "", "staff name (required)")
	role := flag.String("role", "owner", "owner, manager, packer or content_editor")
	flag.Parse()

	if *email == "" || *name == "" {
		flag.Usage()
		os.Exit(2)
	}

	fmt.Printf("Password for %s (at least %d characters): ", *email, staff.MinPasswordLength)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password := strings.TrimRight(line, "\r\n")

	cfg := config.Load()
	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	id, err := staff.NewService(pool).Create(ctx, *email, *name, staff.Role(*role), password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not create the account:", err)
		os.Exit(1)
	}
	fmt.Printf("Created %s account #%d for %s.\n", *role, id, *email)
}
