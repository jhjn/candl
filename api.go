package main

import (
	_ "embed"
	"net/http"
	"regexp"
	"text/template"
)

//go:embed edit.html
var editTemplate string
var editTmpl = template.Must(template.New("edit").Parse(editTemplate))

// A handler for mutating APIs
type Api struct {
	wiki *Wiki
}

// The handler for all wiki pages
func (a *Api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	op := r.PathValue("op")
	if r.Method == "GET" && op == "edit" {
		a.serveGetEdit(w, r)
	} else if op == "edit" {
		a.servePostEdit(w, r)
	}
}

// Serve the edit page for a specific page
func (a *Api) serveGetEdit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	a.wiki.mu.RLock()
	page, ok := a.wiki.Pages[name]
	a.wiki.mu.RUnlock()

	md := ""
	if ok {
		md = page.Raw
	}

	editTmpl.Execute(w, map[string]interface{}{
		"Name":     name,
		"Markdown": md,
	})
}

// Update a page following an edit
// Be careful - without proper validation this could be used to write arbitrary files
func (a *Api) servePostEdit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	body := r.FormValue("body")

	// If page doesn't exist make sure the name is valid!
	if matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", name); err != nil || !matched {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := a.wiki.WritePage(name, body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/"+name, http.StatusSeeOther)
}
