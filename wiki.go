package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// A parsed wiki page.
// Used to serve HTML and understand inter-page linking.
type Page struct {
	// Filled during dir-walk
	Name string // filename relative to wiki dir without .md
	Raw  string // raw markdown
	// Filled after parsing
	Title     string          // from first '#' heading or Name
	HTML      template.HTML   // The converted markdown
	Links     map[string]bool // set of outbound page names
	Backlinks []string        // inbound page names
}

// A collection of parsed markdown pages.
type Wiki struct {
	mu       sync.RWMutex // Used for safe reloads
	Pages    map[string]*Page
	Template *template.Template
	Dir      string // The only required input
}

// regex for wikilinks like [[Some Page]] or [[Some Page|Label]]
var linkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

// Create page data from a directory
func getPages(dir string) (map[string]*Page, error) {
	pages := map[string]*Page{}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".md") {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			page := &Page{
				Name:  strings.TrimSuffix(d.Name(), ".md"),
				Raw:   string(b),
				Links: map[string]bool{},
			}
			pages[page.Name] = page
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// First pass: extract outbound links and render HTML
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	for _, p := range pages {
		// Process wikilinks
		processed := linkRe.ReplaceAllStringFunc(p.Raw, func(m string) string {
			sub := linkRe.FindStringSubmatch(m)
			if len(sub) >= 2 {
				target := strings.TrimSpace(sub[1])
				p.Links[target] = true // Add link to page set

				var label string
				if len(sub) >= 3 {
					label = strings.TrimSpace(sub[2])
				}
				if label == "" {
					label = target
				}
				return fmt.Sprintf("[%s](%s)", label, target)
			}
			return m // Match but not right size... empty [[]]?
		})

		// Render HTML
		var sb strings.Builder
		if err := md.Convert([]byte(processed), &sb); err != nil {
			return nil, err
		}
		p.HTML = template.HTML(sb.String())
	}

	// Build backlinks
	pageLinkers := map[string]map[string]struct{}{}
	for name := range pages {
		pageLinkers[name] = map[string]struct{}{}
	}
	for linker, p := range pages {
		for target := range p.Links {
			if _, ok := pages[target]; ok {
				pageLinkers[target][linker] = struct{}{}
			}
		}
	}
	for name, mset := range pageLinkers {
		arr := []string{}
		for k := range mset {
			arr = append(arr, k)
		}
		pages[name].Backlinks = arr
	}
	return pages, nil
}

// Scan directory for .md files and build pages with backlinks.
// NOTE: Later handle updating the template if it changes.
func (w *Wiki) Update() error {
	pages, err := getPages(w.Dir)
	if err != nil {
		return err
	}
	w.Pages = pages
	return nil
}
