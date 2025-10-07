// candl-wiki
// ----------
// A simple web server for a local wiki directory.
//
// Features:
// - load Markdown files from a directory, converting with
//   - gfm
//   - [[foo]] wikilinks
//   - ::: div fences and {.foo} attrs
// - parse [[wikilinks]] and build and display backlinks
// - edit pages from site
// - create today's diary page
// - page /search will automatically have backlinks from every page
// - watch directory and automatically reload if wiki files change

package main

import (
	_ "embed"
	"flag"
	"log/slog"

	"github.com/jhjn/candl/server"
)

func main() {
	verbose := flag.Bool("v", false, "print debug output")
	dir := flag.String("wiki", ".", "directory containing markdown files")
	port := flag.String("port", "8812", "port to listen on")
	watch := flag.Bool("watch", false, "watch directory for changes")
	flag.Parse()

	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	err := server.Serve(*dir, *port, *watch)
	if err != nil {
		slog.Error("failed to load wiki", "error", err)
	}

}
