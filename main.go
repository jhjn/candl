// lpage wiki
// ----------
//
// - load Markdown files from a directory
// - parse [[wikilinks]] and build backlinks
// - render Markdown -> HTML using goldmark
// - serve pages via net/http with a template
// - optional fsnotify-based watcher to auto-reload

package main

import (
	_ "embed"
	"flag"
	"log/slog"
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

	err := Serve(*dir, *port, *watch)
	if err != nil {
		slog.Error("failed to load wiki", "error", err)
	}

}
