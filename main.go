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
	"log"
	"log/slog"
	"net/http"
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

	r, err := GetServer(*dir, *watch)
	if err != nil {
		log.Fatalf("failed to load wiki: %v", err)
	}

	slog.Info("serving", "wiki", *dir, "port", *port)
	if err := http.ListenAndServe(":"+*port, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
