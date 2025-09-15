package main

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var tmplFS embed.FS

var (
	recipesTmpl = template.Must(template.ParseFS(tmplFS, "templates/base.html", "templates/recipes.html"))
	glyphsTmpl  = template.Must(template.ParseFS(tmplFS, "templates/base.html", "templates/glyphs.html"))
)
