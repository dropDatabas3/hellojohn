package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		configPath = flag.String("config", "configs/config.example.yaml", "Path to YAML config")
		dir        = flag.String("dir", "migrations/postgres", "Migrations directory (contains *_up.sql and *_down.sql)")
	)
	flag.Parse()

	// Positional args: [action] [steps]
	action := "up"
	steps := 0
	args := flag.Args()
	if len(args) >= 1 && args[0] != "" {
		action = strings.ToLower(args[0])
	}
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			steps = n
		}
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Storage.DSN)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	switch action {
	case "up":
		upFiles, err := listSQL(*dir, "_up.sql")
		if err != nil {
			log.Fatalf("list up: %v", err)
		}
		if len(upFiles) == 0 {
			log.Println("No *_up.sql migrations found. Nothing to do.")
			return
		}
		sort.Strings(upFiles) // apply in ascending order
		if steps > 0 && steps < len(upFiles) {
			upFiles = upFiles[:steps]
		}
		log.Printf("Applying %d up migration(s)...", len(upFiles))
		for _, f := range upFiles {
			if err := execSQLFile(ctx, pool, f); err != nil {
				log.Fatalf("exec %s: %v", f, err)
			}
		}
		log.Println("Up migrations completed.")

	case "down":
		downFiles, err := listSQL(*dir, "_down.sql")
		if err != nil {
			log.Fatalf("list down: %v", err)
		}
		if len(downFiles) == 0 {
			log.Println("No *_down.sql migrations found. Nothing to do.")
			return
		}
		sort.Strings(downFiles)   // ascending
		reverseInPlace(downFiles) // run in reverse
		if steps > 0 && steps < len(downFiles) {
			downFiles = downFiles[:steps] // only N most-recent downs
		}
		log.Printf("Applying %d down migration(s)...", len(downFiles))
		for _, f := range downFiles {
			if err := execSQLFile(ctx, pool, f); err != nil {
				log.Fatalf("exec %s: %v", f, err)
			}
		}
		log.Println("Down migrations completed.")

	default:
		log.Fatalf("unknown action %q. Use: up | down [steps]", action)
	}
}

func listSQL(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			name := e.Name()
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
				out = append(out, filepath.Join(dir, name))
			}
		}
	}
	return out, nil
}

func reverseInPlace(ss []string) {
	for i, j := 0, len(ss)-1; i < j; i, j = i+1, j-1 {
		ss[i], ss[j] = ss[j], ss[i]
	}
}

func execSQLFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	sql := string(b)

	start := time.Now()
	_, err = pool.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	log.Printf("OK %s (%s)", filepath.Base(path), time.Since(start).Truncate(time.Millisecond))
	return nil
}
