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
	"context"
	_ "embed"
	"flag"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Server holds the wiki and template.
type Server struct {
	wiki *Wiki
}

// defaultTemplate is used if template.html not found in wiki dir.
//
//go:embed template.html
var defaultTemplate string

// defaultStyle is used if style.css not found in wiki dir.
//
//go:embed style.css
var defaultStyle string

func NewWiki(dir string) (*Wiki, error) {
	templ, err := getTemplate(dir)
	if err != nil {
		return nil, err
	}
	return &Wiki{
		Pages:    map[string]*Page{},
		Template: templ,
		Dir:      dir,
	}, nil
}

// Get template from $WIKI/template.html or use embedded default.
func getTemplate(dir string) (*template.Template, error) {
	p := filepath.Join(dir, "template.html")
	var src string
	if _, err := os.Stat(p); err == nil {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		src = string(b)
	} else {
		src = defaultTemplate
	}
	tmpl, err := template.New("page").Parse(src)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Get style from $WIKI/style.css or use embedded default.
func GetStyle(dir string) (string, error) {
	p := filepath.Join(dir, "style.css")
	if _, err := os.Stat(p); err == nil {
		b, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return defaultStyle, nil
}

// The handler for all wiki pages
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var name string
	if r.URL.Path == "/" {
		name = "index"
	} else {
		name = r.URL.Path[1:]
	}

	s.wiki.mu.RLock()
	page, ok := s.wiki.Pages[name]
	s.wiki.mu.RUnlock()
	// NOTE: Is it ok to unlock at this point? Couldn't page be edited or is that fine?
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := s.wiki.Template.Execute(w, map[string]interface{}{
		"Title":     page.Title,
		"Content":   page.HTML,
		"Backlinks": page.Backlinks,
	}); err != nil {
		log.Printf("template error: %v", err)
	}
}

// WatchDir: watches directory and reloads wiki on changes.
func WatchDir(ctx context.Context, wiki *Wiki) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// add directory and subdirs (non-recursive for simplicity)
	// NOTE: Won't work for subdirs
	if err := watcher.Add(wiki.Dir); err != nil {
		return err
	}

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// We debounce rapid events
			_ = ev
			debounce.Reset(200 * time.Millisecond)
		case <-debounce.C:
			if err := wiki.Update(); err != nil {
				log.Printf("reload error: %v", err)
				continue
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Println("watcher error:", err)
		}
	}
}

func main() {
	verbose := flag.Bool("v", false, "print debug output")
	dir := flag.String("wiki", ".", "directory containing markdown files")
	port := flag.String("port", "8812", "port to listen on")
	watch := flag.Bool("watch", false, "watch directory for changes")
	flag.Parse()

	if *verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	wiki, err := NewWiki(*dir)
	if err != nil {
		log.Fatalf("failed to load wiki: %v", err)
	}

	if err := wiki.Update(); err != nil {
		log.Fatalf("failed to load wiki: %v", err)
	}

	style, err := GetStyle(*dir)
	if err != nil {
		log.Fatalf("failed to load style: %v", err)
	}

	r := http.NewServeMux()
	r.Handle("/", &Server{wiki: wiki})
	r.Handle("/style.css", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(style))
	}))

	if *watch {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go WatchDir(ctx, wiki)
	}

	slog.Info("serving", "wiki", *dir, "port", *port)
	if err := http.ListenAndServe(":"+*port, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
