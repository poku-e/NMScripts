package main

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var tmplFS embed.FS

var (
	indexTmpl  = template.Must(template.ParseFS(tmplFS, "templates/index.html"))
	glyphsTmpl = template.Must(template.ParseFS(tmplFS, "templates/glyphs.html"))
)
