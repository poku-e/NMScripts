package main

import (
	"flag"
	"log"
	"path/filepath"
)

// ---------- Main ----------

func main() {
	var foodPath string
	var refinerPath string
	var addr string
	var glyphPath string

	flag.StringVar(&foodPath, "csv", "food.csv", "Path to food.csv (recipe table)")
	flag.StringVar(&refinerPath, "refiner", "refiner.csv", "Path to refiner.csv (recipe table)")
	flag.StringVar(&addr, "addr", ":8080", "Listen address")
	flag.StringVar(&glyphPath, "glyphs", "glyphs.json", "Path to glyphs JSON file")
	flag.Parse()

	if !filepath.IsAbs(foodPath) {
		if abs, err := filepath.Abs(foodPath); err == nil {
			foodPath = abs
		}
	}
	if !filepath.IsAbs(refinerPath) {
		if abs, err := filepath.Abs(refinerPath); err == nil {
			refinerPath = abs
		}
	}
	if !filepath.IsAbs(glyphPath) {
		if abs, err := filepath.Abs(glyphPath); err == nil {
			glyphPath = abs
		}
	}

	foodDB, err := loadCSV(foodPath)
	if err != nil {
		log.Fatalf("load food csv: %v", err)
	}
	if len(foodDB.Recipes) == 0 {
		log.Fatalf("no recipes parsed from %s", foodPath)
	}

	refDB, err := loadCSV(refinerPath)
	if err != nil {
		log.Fatalf("load refiner csv: %v", err)
	}
	if len(refDB.Recipes) == 0 {
		log.Fatalf("no refiner recipes parsed from %s", refinerPath)
	}

	gs := &GlyphStore{Path: glyphPath}
	if err := gs.Load(); err != nil {
		log.Fatalf("load glyphs: %v", err)
	}

	log.Printf("food recipes: %d | ingredients: %d | csv: %s", len(foodDB.Recipes), len(foodDB.AllIngredients), foodPath)
	log.Printf("refiner recipes: %d | ingredients: %d | csv: %s", len(refDB.Recipes), len(refDB.AllIngredients), refinerPath)
	log.Printf("glyphs: %d | file: %s", len(gs.Items), glyphPath)

	if err := serve(foodDB, refDB, gs, addr); err != nil {
		log.Fatal(err)
	}
}
