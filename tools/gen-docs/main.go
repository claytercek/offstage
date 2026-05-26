// Package main generates man pages for the offstage CLI.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/claytercek/offstage/internal/cli"
)

func main() {
	dir := "man"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("create output directory: %v", err)
	}

	header := &doc.GenManHeader{
		Title:   "OFFSTAGE",
		Section: "1",
	}

	root := cli.NewRootCmd("")
	// Disable the default completion command from appearing as its own man page
	// since it's a helper, not a primary command. Users still get bash/zsh/fish/powershell
	// completions via `offstage completion <shell>`.
	if err := doc.GenManTree(root, header, dir); err != nil {
		log.Fatalf("generate man pages: %v", err)
	}

	fmt.Printf("man pages written to %s/\n", dir)
}
