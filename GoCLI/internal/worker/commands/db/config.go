package db

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agentsynch/internal/config"
)

func Config() {
	flags := flag.NewFlagSet("config", flag.ExitOnError)
	vaultFlag := flags.String("vault", "", "path to Obsidian vault")
	dbFlag := flags.String("db", "", "path to task database")
	flags.Parse(os.Args[2:])

	if *vaultFlag == "" && *dbFlag == "" {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
		if cfg.VaultPath == "" {
			fmt.Println("vault: (not set)")
		} else {
			fmt.Printf("vault: %s\n", cfg.VaultPath)
		}
		if cfg.DBPath == "" {
			fmt.Println("db: (default ~/.agentsynch/tasks.db)")
		} else {
			fmt.Printf("db: %s\n", cfg.DBPath)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	if *vaultFlag != "" {
		// expand ~/
		vault := *vaultFlag
		if strings.HasPrefix(vault, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error resolving home directory: %v\n", err)
				os.Exit(1)
			}
			vault = filepath.Join(home, vault[2:])
		}
		cfg.VaultPath = vault
		fmt.Printf("vault set to: %s\n", vault)
	}

	if *dbFlag != "" {
		// expand ~/
		db := *dbFlag
		if strings.HasPrefix(db, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error resolving home directory: %v\n", err)
				os.Exit(1)
			}
			db = filepath.Join(home, db[2:])
		}
		cfg.DBPath = db
		fmt.Printf("db set to: %s\n", db)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}
}
