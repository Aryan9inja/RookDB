package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/Aryan9inja/RookDB/internal/engine"
	"github.com/Aryan9inja/RookDB/internal/parser"
)

const version = "1.0.0"

const helpText = `RookDB v1.0.0

Usage:
  rookdb <data-directory>

Options:
  -h, --help       Show this help message
  -v, --version    Show RookDB version

Commands:
  SET <key> <value>
  GET <key>
  DELETE <key>

Controls:
  Ctrl+D      Exit RookDB
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-directory>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	if os.Args[1] == "-v" || os.Args[1] == "--version" {
		fmt.Printf("RookDB v%s\n", version)
		os.Exit(0)
	}
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Print(helpText)
		os.Exit(0)
	}

	fmt.Printf("RookDB v%s\n", version)

	dataDir := os.Args[1]

	databaseEngine, err := engine.NewEngine(filepath.Join(dataDir, "rookdb.wal"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database engine initialization failed with error: %v\n", err)
		os.Exit(1)
	}

	databaseParser := parser.NewParser(os.Stdin)

	for {
		fmt.Printf(">> ")
		cmd, key, value, err := databaseParser.ReadCommand()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// ctrl + D pressed
				break
			}

			// Client/command error.
			fmt.Println(err)
			continue
		}

		data, err := databaseEngine.Execute(cmd, key, value)
		if err != nil {
			if errors.Is(err, engine.ErrStorageFailure) {
				// Fatal: database cannot safely continue.
				break
			}

			// Client/command error.
			fmt.Println(err)
			continue
		}

		if data == "" {
			fmt.Println("OK")
		} else {
			fmt.Println(data)
		}
	}

	// clean exit
	if err := databaseEngine.Close(); err != nil {
		log.Printf("couldn't exit cleanly: %v", err)
	}
}
