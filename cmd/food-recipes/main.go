package main

import (
	"flag"
	"log"
	"path/filepath"
)

// ---------- Main ----------

func main() {
	var csvPath string
	var addr string
	var glyphPath string

	flag.StringVar(&csvPath, "csv", "food.csv", "Path to food.csv (recipe table)")
	flag.StringVar(&addr, "addr", ":8080", "Listen address")
	flag.StringVar(&glyphPath, "glyphs", "glyphs.json", "Path to glyphs JSON file")
	flag.Parse()

	if !filepath.IsAbs(csvPath) {
		if abs, err := filepath.Abs(csvPath); err == nil {
			csvPath = abs
		}
	}
	if !filepath.IsAbs(glyphPath) {
		if abs, err := filepath.Abs(glyphPath); err == nil {
			glyphPath = abs
		}
	}

	db, err := loadCSV(csvPath)
	if err != nil {
		log.Fatalf("load csv: %v", err)
	}
	if len(db.Recipes) == 0 {
		log.Fatalf("no recipes parsed from %s", csvPath)
	}

	gs := &GlyphStore{Path: glyphPath}
	if err := gs.Load(); err != nil {
		log.Fatalf("load glyphs: %v", err)
	}

	log.Printf("recipes: %d | ingredients: %d | csv: %s", len(db.Recipes), len(db.AllIngredients), csvPath)
	log.Printf("glyphs: %d | file: %s", len(gs.Items), glyphPath)

	if err := serve(db, gs, addr); err != nil {
		log.Fatal(err)
	}
}
