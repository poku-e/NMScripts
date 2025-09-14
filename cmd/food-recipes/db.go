package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---------- Data model: Recipes ----------

type Recipe struct {
	Inputs []string `json:"inputs"`
	Output string   `json:"output"`
	Qty    int      `json:"qty"`
}

type DB struct {
	Recipes         []Recipe
	AllIngredients  []string
	ingIndex        map[string][]int // ingredient -> indices into Recipes
	normIngToActual map[string]string
}

// ---------- CSV load ----------

func loadCSV(path string) (*DB, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer func(f *os.File) {
		if cerr := f.Close(); cerr != nil {
			fmt.Printf("error closing file: %v", cerr)
		}
	}(f)

	cr := csv.NewReader(f)
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv has no rows")
	}

	headers := map[string]int{}
	for i, h := range records[0] {
		headers[strings.TrimSpace(strings.ToLower(h))] = i
	}

	col := func(name string) (int, bool) {
		i, ok := headers[strings.ToLower(name)]
		return i, ok
	}

	req := []string{
		"input1_name", "input2_name", "input3_name",
		"output_name", "output_qty",
	}
	for _, r := range req {
		if _, ok := col(r); !ok {
			return nil, fmt.Errorf("missing required column: %s", r)
		}
	}

	var db DB
	db.ingIndex = make(map[string][]int)
	db.normIngToActual = make(map[string]string)
	ingSet := make(map[string]struct{})

	for r := 1; r < len(records); r++ {
		row := records[r]
		if len(row) == 0 {
			continue
		}
		var inputs []string
		for _, name := range []string{"input1_name", "input2_name", "input3_name"} {
			if idx, ok := col(name); ok && idx < len(row) {
				if v := strings.TrimSpace(row[idx]); v != "" {
					inputs = append(inputs, v)
				}
			}
		}
		var output string
		if idx, ok := col("output_name"); ok && idx < len(row) {
			output = strings.TrimSpace(row[idx])
		}
		if output == "" || len(inputs) == 0 {
			continue
		}
		qty := 1
		if idx, ok := col("output_qty"); ok && idx < len(row) {
			if q, err := strconv.Atoi(strings.TrimSpace(row[idx])); err == nil && q > 0 {
				qty = q
			}
		}
		rec := Recipe{Inputs: inputs, Output: output, Qty: qty}
		db.Recipes = append(db.Recipes, rec)
	}

	for i, rec := range db.Recipes {
		_ = i
		for _, ing := range rec.Inputs {
			ing = strings.TrimSpace(ing)
			if ing == "" {
				continue
			}
			ingSet[ing] = struct{}{}
			db.ingIndex[ing] = append(db.ingIndex[ing], i)
			db.normIngToActual[normKey(ing)] = ing
		}
	}

	for ing := range ingSet {
		db.AllIngredients = append(db.AllIngredients, ing)
	}
	sort.Strings(db.AllIngredients)

	return &db, nil
}

// ---------- Fuzzy matching helpers ----------

func normKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) || unicode.IsPunct(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func lev(a, b string) int {
	if a == b {
		return 0
	}
	la := utf8.RuneCountInString(a)
	lb := utf8.RuneCountInString(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	ar := []rune(a)
	br := []rune(b)

	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			a := prev[j] + 1
			b := cur[j-1] + 1
			c := prev[j-1] + cost
			cur[j] = min(a, min(b, c))
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type match struct {
	Actual string
	Score  float64
}

func (db *DB) mapUserIngredients(inputs []string) ([]string, []string) {
	var mapped []string
	var unknown []string

	type cand struct{ norm, actual string }
	candidates := make([]cand, 0, len(db.AllIngredients))
	for _, ing := range db.AllIngredients {
		candidates = append(candidates, cand{norm: normKey(ing), actual: ing})
	}

	for _, raw := range inputs {
		q := normKey(raw)
		if q == "" {
			continue
		}
		if act, ok := db.normIngToActual[q]; ok {
			mapped = append(mapped, act)
			continue
		}
		best := match{"", math.MaxFloat64}
		for _, c := range candidates {
			d := float64(lev(q, c.norm))
			if strings.Contains(c.norm, q) || strings.Contains(q, c.norm) {
				d *= 0.5
			}
			if d < best.Score {
				best = match{Actual: c.actual, Score: d}
			}
		}
		if best.Actual != "" && best.Score <= 2.5 {
			mapped = append(mapped, best.Actual)
		} else {
			unknown = append(unknown, raw)
		}
	}
	seen := map[string]struct{}{}
	uniq := mapped[:0]
	for _, m := range mapped {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		uniq = append(uniq, m)
	}
	return uniq, unknown
}

func (db *DB) suggest(all []string) []Recipe {
	if len(all) == 0 {
		return nil
	}
	var idxs []int
	for i, ing := range all {
		list := db.ingIndex[ing]
		if i == 0 {
			idxs = append([]int(nil), list...)
			continue
		}
		idxs = intersectSortedOrUnsorted(idxs, list)
		if len(idxs) == 0 {
			break
		}
	}
	seen := map[int]struct{}{}
	out := make([]Recipe, 0, len(idxs))
	for _, ix := range idxs {
		if _, ok := seen[ix]; ok {
			continue
		}
		seen[ix] = struct{}{}
		out = append(out, db.Recipes[ix])
	}
	return out
}

func intersectSortedOrUnsorted(a, b []int) []int {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	ac := append([]int(nil), a...)
	bc := append([]int(nil), b...)
	sort.Ints(ac)
	sort.Ints(bc)
	var out []int
	i, j := 0, 0
	for i < len(ac) && j < len(bc) {
		switch {
		case ac[i] == bc[j]:
			out = append(out, ac[i])
			i++
			j++
		case ac[i] < bc[j]:
			i++
		default:
			j++
		}
	}
	return out
}
