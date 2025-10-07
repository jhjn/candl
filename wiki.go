package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"

	fences "github.com/stefanfritsch/goldmark-fences"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
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

func sortBacklinks(a, b string) int {
	// Check if strings start with digits
	aBeginsNum := len(a) > 0 && unicode.IsDigit(rune(a[0]))
	bBeginsNum := len(b) > 0 && unicode.IsDigit(rune(b[0]))

	if !aBeginsNum && bBeginsNum {
		return -1 // a (alpha) comes before b (numeric)
	}
	if aBeginsNum && !bBeginsNum {
		return 1 // b (alpha) comes before a (numeric)
	}
	// Both are alphabetic - normal sort
	if !aBeginsNum && !bBeginsNum {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}

	// Both are numeric - reverse sort (highest to lowest)
	if aBeginsNum && bBeginsNum {
		if a < b {
			return 1
		}
		if a > b {
			return -1
		}
		return 0
	}

	return 0 // Should never reach here
}

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
	// Add /search page if it doesn't exist
	if _, ok := pages["search"]; !ok {
		pages["search"] = &Page{
			Name: "search",
			Raw:  "# Search",
		}
	}

	// First pass: extract outbound links and render HTML
	// NOTE: In future add https://github.com/yuin/goldmark-highlighting
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, &fences.Extender{}),
		goldmark.WithParserOptions(parser.WithAttribute()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	for _, p := range pages {
		// Process title (if '# ' get string until newline)
		if strings.HasPrefix(p.Raw, "# ") && strings.Index(p.Raw, "\n") > 0 {
			p.Title = strings.TrimSpace(p.Raw[2:strings.Index(p.Raw, "\n")])
		}
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
		// Every page implicitly links to 'search'
		pageLinkers["search"][linker] = struct{}{}
	}
	for name, mset := range pageLinkers {
		arr := []string{}
		for k := range mset {
			arr = append(arr, k)
		}
		pages[name].Backlinks = arr
		slices.SortFunc(pages[name].Backlinks, sortBacklinks)
	}
	return pages, nil
}

// Scan directory for .md files and build pages with backlinks.
// NOTE: Later handle updating the template if it changes.
// NOTE: Implement the updating of single files!
func (w *Wiki) Update() error {
	pages, err := getPages(w.Dir)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.Pages = pages
	w.mu.Unlock()
	return nil
}

func (w *Wiki) WritePage(name string, content string) error {
	path := filepath.Join(w.Dir, name+".md")
	return os.WriteFile(path, []byte(content), 0644)
}
