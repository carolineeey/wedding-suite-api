// Command admin manages admin accounts from the operator's shell. There is
// no signup or password-reset endpoint (and no email delivery to back one),
// so accounts are created, granted weddings, and reset here, against
// DATABASE_URL.
//
//	go run ./cmd/admin create -email you@example.com [-wedding slug]
//	go run ./cmd/admin grant  -email you@example.com -wedding slug
//	go run ./cmd/admin passwd -email you@example.com
//
// Passwords are prompted for without echo, or read from the first line of
// stdin when it is not a terminal.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/config"
	"github.com/carolineeey/wedding-suite-api/internal/db"
	"github.com/carolineeey/wedding-suite-api/internal/repository"
	"github.com/carolineeey/wedding-suite-api/internal/usecase"
	"github.com/joho/godotenv"
	"golang.org/x/term"
)

const usage = `usage:
  admin create -email EMAIL [-wedding SLUG]   create an admin, optionally granting a wedding
  admin grant  -email EMAIL -wedding SLUG     let an existing admin manage a wedding
  admin passwd -email EMAIL                   set a new password and log out all sessions`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	email := fs.String("email", "", "admin email")
	wedding := fs.String("wedding", "", "wedding slug")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("-email is required\n" + usage)
	}

	_ = godotenv.Load()
	cfg := config.Load()
	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close()

	weddingRepo := repository.NewWeddingRepository(conn)
	auth := usecase.NewAuthUsecase(
		repository.NewAdminRepository(conn),
		weddingRepo,
		repository.NewSessionRepository(conn),
		usecase.NewWeddingScope(weddingRepo),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch cmd {
	case "create":
		password, err := readPassword()
		if err != nil {
			return err
		}
		admin, err := auth.CreateAdmin(ctx, *email, password)
		if err != nil {
			return err
		}
		fmt.Printf("created admin %s (%s)\n", admin.Email, admin.ID)
		if *wedding == "" {
			return nil
		}
		if err := auth.Grant(ctx, *email, *wedding); err != nil {
			return err
		}
		fmt.Printf("granted %s access to %s\n", admin.Email, *wedding)

	case "grant":
		if *wedding == "" {
			return errors.New("-wedding is required\n" + usage)
		}
		if err := auth.Grant(ctx, *email, *wedding); err != nil {
			return err
		}
		fmt.Printf("granted %s access to %s\n", *email, *wedding)

	case "passwd":
		password, err := readPassword()
		if err != nil {
			return err
		}
		if err := auth.SetPassword(ctx, *email, password); err != nil {
			return err
		}
		fmt.Printf("password updated for %s; existing sessions logged out\n", *email)

	default:
		return fmt.Errorf("unknown command %q\n%s", cmd, usage)
	}
	return nil
}

// readPassword prompts twice on a terminal, so a typo does not lock the
// admin out; piped input is taken as is.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("reading password from stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(os.Stderr, "password: ")
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "confirm password: ")
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(first) != string(second) {
		return "", errors.New("passwords do not match")
	}
	return string(first), nil
}
