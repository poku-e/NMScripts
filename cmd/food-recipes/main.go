package main

import (
	"flag"
	"log"
	"path/filepath"
)

// ---------- Main ----------

func absPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func main() {
	var foodPath, refinerPath, addr, glyphPath string

	flag.StringVar(&foodPath, "csv", "food.csv", "Path to food.csv (recipe table)")
	flag.StringVar(&refinerPath, "refiner", "refiner.csv", "Path to refiner.csv (recipe table)")
	flag.StringVar(&addr, "addr", ":8080", "Listen address")
	flag.StringVar(&glyphPath, "glyphs", "glyphs.json", "Path to glyphs JSON file")
	flag.Parse()

	foodPath = absPath(foodPath)
	refinerPath = absPath(refinerPath)
	glyphPath = absPath(glyphPath)

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
