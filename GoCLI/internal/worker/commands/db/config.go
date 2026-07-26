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
	flags.Parse(os.Args[2:])

	if *vaultFlag == "" {
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
		return
	}

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

	cfg := config.Config{VaultPath: vault}
	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("vault set to: %s\n", vault)
}
