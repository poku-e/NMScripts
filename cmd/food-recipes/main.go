package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---------- Data model ----------

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

	// Header indices
	headers := map[string]int{}
	for i, h := range records[0] {
		headers[strings.TrimSpace(strings.ToLower(h))] = i
	}

	col := func(name string) (int, bool) {
		i, ok := headers[strings.ToLower(name)]
		return i, ok
	}

	// Minimal required columns
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

	// Rows
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
			// skip malformed rows
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

	// Build ingredient sets and index
	for i, rec := range db.Recipes {
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

// ---------- Fuzzy matching ----------

// Normalize to a form usable for matching: lower, strip diacritics, collapse spaces
func normKey(s string) string {
	// lower + trim
	s = strings.ToLower(strings.TrimSpace(s))
	// remove diacritics & non-letters/digits/spaces
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// strip combining marks
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		// keep letters, numbers, spaces
		if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	ns := strings.Join(strings.Fields(b.String()), " ")
	return ns
}

// Levenshtein distance (iterative, O(min(m,n)) memory)
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
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			cur[j] = min3(del, ins, sub)
		}
		copy(prev, cur)
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

type match struct {
	Actual string
	Score  float64 // smaller distance -> higher score; transform to similarity
}

// map each user entry to best ingredient in DB
func (db *DB) mapUserIngredients(inputs []string) ([]string, []string) {
	var mapped []string
	var unknown []string

	// Precompute normalized candidate keys
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
		// Exact normalized match first
		if act, ok := db.normIngToActual[q]; ok {
			mapped = append(mapped, act)
			continue
		}
		// Fuzzy: choose minimal Levenshtein distance with small penalty for length
		best := match{"", math.MaxFloat64}
		for _, c := range candidates {
			d := float64(lev(q, c.norm))
			// Slight bias for substring containments
			if strings.Contains(c.norm, q) || strings.Contains(q, c.norm) {
				d *= 0.5
			}
			if d < best.Score {
				best = match{Actual: c.actual, Score: d}
			}
		}
		// Threshold: accept if reasonably close (tuned for short names)
		if best.Actual != "" && best.Score <= 2.5 {
			mapped = append(mapped, best.Actual)
		} else {
			unknown = append(unknown, raw)
		}
	}
	// de-duplicate while preserving order
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

// Suggest recipes that include all mapped ingredients
func (db *DB) suggest(all []string) []Recipe {
	if len(all) == 0 {
		return nil
	}
	// Intersect indices over ingredients
	var idxs []int
	for i, ing := range all {
		list := db.ingIndex[ing]
		if i == 0 {
			idxs = append([]int(nil), list...)
			continue
		}
		// intersect with existing idxs
		idxs = intersectSortedOrUnsorted(idxs, list)
		if len(idxs) == 0 {
			break
		}
	}
	// unique and preserve order
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
	// if not sorted, sorting is O(n log n) and amortized fine here
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

// ---------- HTTP (API + UI) ----------

type apiResp struct {
	Mapped       []string `json:"mapped"`
	Unrecognized []string `json:"unrecognized"`
	Suggestions  []Recipe `json:"suggestions"`
}

func serve(db *DB, addr string) error {
	mux := http.NewServeMux()

	// API: /api/suggest?have=ing1,ing2,...
	mux.HandleFunc("/api/suggest", func(w http.ResponseWriter, r *http.Request) {
		have := strings.TrimSpace(r.URL.Query().Get("have"))
		if have == "" {
			http.Error(w, "missing 'have' query param", http.StatusBadRequest)
			return
		}
		parts := splitCSVLike(have)
		mapped, unknown := db.mapUserIngredients(parts)
		sugs := db.suggest(mapped)

		resp := apiResp{
			Mapped:       mapped,
			Unrecognized: unknown,
			Suggestions:  sugs,
		}
		writeJSON(w, resp)
	})

	// API: /api/ingredients (for client-side suggestions/autocomplete)
	mux.HandleFunc("/api/ingredients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, db.AllIngredients)
	})

	// UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var buf bytes.Buffer
		if err := indexTmpl.Execute(&buf, nil); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "error writing response: %v\n", err)
			return
		}
	})

	log.Printf("listening on %s", addr)
	return http.ListenAndServe(addr, withCommonHeaders(mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

func withCommonHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow local dev from any origin if you embed somewhere
		w.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(w, r)
	})
}

var csvSplitter = regexp.MustCompile(`[,\n;]+`)

func splitCSVLike(s string) []string {
	raw := csvSplitter.Split(s, -1)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------- Template (Glassy iOS Mint UI + token input + fixed Enter) ----------

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Nirvana Recipe Finder</title>
<style>
:root{
  /* Darker, richer mint scale */
  --mint-25:#daf7ee; --mint-50:#c6f2e6; --mint-100:#a2ecd9;
  --mint-200:#7de6cc; --mint-300:#58dfbf; --mint-400:#35d9b3;
  --mint-500:#22d8ad; --mint-600:#17b392; --mint-700:#118b73;
  --bg-dark-1:#0c2924; --bg-dark-2:#0e312b;
  --glass-tint:rgba(20,50,45,0.30);
  --text-900:#e9fffa; --text-700:#b6e6d9; --text-500:#8dd7c6;
}

*{box-sizing:border-box}
html,body{height:100%;margin:0;font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial}

/* Background (darker + mint diagonal stripes) */
body{
  color:var(--text-900);
  background:
    radial-gradient(900px 520px at 15% -10%, rgba(53,217,179,0.18), transparent 55%),
    radial-gradient(800px 480px at 110% 20%, rgba(88,223,191,0.14), transparent 50%),
    repeating-linear-gradient(45deg,
      rgba(53,217,179,0.08) 0px, rgba(53,217,179,0.08) 14px,
      rgba(17,139,115,0.10) 14px, rgba(17,139,115,0.10) 28px),
    linear-gradient(180deg, var(--bg-dark-1) 0%, var(--bg-dark-2) 100%);
}

.container{ min-height:100%; display:flex; align-items:center; justify-content:center; padding:24px; }

/* Glass panel */
.card{
  width:min(900px,92vw); position:relative;
  backdrop-filter: blur(26px) saturate(120%); -webkit-backdrop-filter: blur(26px) saturate(120%);
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06)), var(--glass-tint);
  border:1px solid rgba(180,255,237,0.18); border-radius:24px; padding:28px;
  box-shadow:0 24px 60px rgba(12,41,36,0.55), inset 0 1px 0 rgba(255,255,255,0.06);
}

/* Header */
.header{display:flex;align-items:center;gap:14px;margin-bottom:12px}
.badge{
  background:linear-gradient(145deg,var(--mint-500),var(--mint-300));
  color:white;font-weight:700;border-radius:12px;padding:6px 10px;font-size:12px;
  box-shadow:0 8px 20px rgba(34,216,173,0.40);
}
h1{font-size:24px;margin:0}
.sub{color:var(--text-700);margin:6px 0 18px 0}

/* === Tokenized input === */
.inputRow{position:relative; display:flex; gap:10px; flex-wrap:wrap}
.tokenBox{
  flex:1; min-height:50px; display:flex; align-items:center; flex-wrap:wrap; gap:8px;
  padding:8px 10px; border-radius:16px; border:1px solid rgba(255,255,255,0.10);
  background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05));
}
.token{
  display:flex; align-items:center; gap:8px; padding:6px 10px; border-radius:999px;
  background:rgba(53,217,179,0.18); border:1px solid rgba(53,217,179,0.35); color:#dffef7;
  max-width:100%;
}
.token .text{white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:220px}
.token .x{
  border:none; background:transparent; color:#eafff9; opacity:.85; cursor:pointer; font-weight:700;
}
.tokenInput{
  flex:1; min-width:160px; border:none; outline:none; background:transparent; color:var(--text-900);
  padding:8px 6px; font-size:16px;
}
.tokenInput::placeholder{color:var(--text-500)}

/* Primary button */
button.primary{
  background:linear-gradient(180deg, var(--mint-500), var(--mint-600));
  color:white;font-weight:700;border:none;border-radius:14px;
  padding:12px 18px;cursor:pointer;
  box-shadow:0 14px 30px rgba(34,216,173,0.35);
  transition:transform .06s ease, box-shadow .2s ease, filter .2s, opacity .2s;
}
button.primary:hover{filter:saturate(110%); box-shadow:0 16px 36px rgba(34,216,173,0.45)}
button.primary:active{transform:translateY(1px); opacity:.95}

/* Autocomplete dropdown */
.dropdown{
  position:absolute; left:0; right:150px; top:100%; z-index:20; margin-top:8px;
  border-radius:14px; overflow:hidden; border:1px solid rgba(255,255,255,0.10);
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06));
  backdrop-filter: blur(18px); -webkit-backdrop-filter: blur(18px);
  box-shadow:0 18px 40px rgba(0,0,0,0.30);
  max-height:280px; overflow-y:auto;
}
.item{
  padding:10px 12px; cursor:pointer; color:var(--text-900);
  border-bottom:1px solid rgba(255,255,255,0.06);
}
.item:last-child{border-bottom:none}
.item:hover, .item.active{
  background:rgba(53,217,179,0.18);
}

/* Chips row (quick add) + footer */
.aux{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:8px}
.chips{display:flex;gap:8px;flex-wrap:wrap}
.chip{
  padding:6px 10px;border-radius:999px;font-size:12px;
  background:rgba(53,217,179,0.14); border:1px solid rgba(53,217,179,0.35); color:#dffef7
}
.footer{margin-top:16px;color:var(--text-700);font-size:12px;text-align:right}
kbd{
  background:rgba(53,217,179,0.20); border-radius:6px; border:1px solid rgba(53,217,179,0.45);
  padding:2px 6px; color:#eafff9
}

/* Results */
.result{
  margin-top:18px; border-radius:18px; padding:16px;
  background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05));
  border:1px solid rgba(255,255,255,0.10);
}
.result h2{font-size:16px;margin:0 0 12px 0;color:#c9fff3}
.list{display:grid;grid-template-columns:1fr;gap:10px}
@media(min-width:720px){.list{grid-template-columns:1fr 1fr}}
.cardItem{
  border-radius:16px;padding:12px 14px;
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06));
  border:1px solid rgba(255,255,255,0.10);
  box-shadow:0 6px 18px rgba(0,0,0,0.18); color:var(--text-900);
}
.itemTitle{font-weight:700;margin-bottom:6px}
.itemMeta{color:var(--text-700);font-size:13px}

/* Warning pill */
.warn{
  color:#ffdede; background:rgba(255,61,61,0.12);
  border:1px solid rgba(255,61,61,0.25); padding:8px 10px; border-radius:10px; margin-top:10px;
}
</style>


</head>
<body>
<div class="container">
  <div class="card">
    <div class="header">
      <span class="badge">Nirvana</span>
      <h1>Recipe Finder</h1>
    </div>
    <div class="sub">Type one or more ingredients. Press <strong>Enter</strong> to add; with the input empty, <strong>Enter</strong> searches.</div>

    <div class="inputRow">
      <div class="tokenBox" id="tokenBox" aria-haspopup="listbox" aria-expanded="false">
        <div id="tokens"></div>
        <input id="ingInput" class="tokenInput" type="text" autocomplete="off"
               placeholder="Type an ingredient and press Enter…" />
      </div>
      <button class="primary" id="btn">Suggest</button>

      <!-- Autocomplete dropdown -->
      <div class="dropdown" id="dropdown" role="listbox" hidden></div>
    </div><br>
    <div class="aux">
      <div class="chips" id="chips"></div>
      <div class="footer">Tip: Enter = add, Enter again = search • ⌘/Ctrl+Enter = add & search</div>
    </div>

    <div class="result" id="result" style="display:none">
      <h2>Suggestions</h2>
      <div id="mapped" class="itemMeta"></div><br>
      <div id="unknown" class="warn" style="display:none"></div>
      <div class="list" id="list"></div>
    </div>
  </div>
</div>
<script>
let ALL_ING = [];
const tokens = []; // selected ingredients

const el = (id) => document.getElementById(id);
const tokenBox = el('tokenBox');
const tokensWrap = el('tokens');
const input = el('ingInput');
const dropdown = el('dropdown');
const suggestBtn = el('btn');

function uniquePush(arr, v){ if(!arr.includes(v)) arr.push(v); }
function removeAt(arr, i){ arr.splice(i, 1); }
function renderTokens(){
  tokensWrap.innerHTML = '';
  tokens.forEach((t,i)=>{
    const d = document.createElement('div'); d.className='token';
    const span = document.createElement('span'); span.className='text'; span.textContent=t;
    const x = document.createElement('button'); x.className='x'; x.type='button'; x.setAttribute('aria-label', 'Remove'); x.textContent='×';
    x.onclick = () => { removeAt(tokens, i); renderTokens(); };
    d.appendChild(span); d.appendChild(x);
    tokensWrap.appendChild(d);
  });
  input.placeholder = tokens.length ? '' : 'Type an ingredient and press Enter…';
  tokenBox.setAttribute('aria-expanded', !dropdown.hidden ? 'true' : 'false');
}

/* --- Autocomplete --- */
let activeIndex = -1; // keyboard focus in dropdown
function filterSuggestions(q){
  const s = q.trim().toLowerCase();
  if(!s) return [];
  // ranking: prefix > substring; exclude already-selected tokens
  const cand = ALL_ING.filter(x => !tokens.includes(x));
  const pref = [], sub = [];
  cand.forEach(c=>{
    const lc = c.toLowerCase();
    if(lc.startsWith(s)) pref.push(c);
    else if(lc.includes(s)) sub.push(c);
  });
  return pref.concat(sub).slice(0, 50);
}
function renderDropdown(items){
  dropdown.innerHTML = '';
  if(items.length === 0){
    dropdown.hidden = true; activeIndex = -1; return;
  }
  items.forEach((text, idx)=>{
    const it = document.createElement('div');
    it.className = 'item' + (idx===activeIndex ? ' active' : '');
    it.setAttribute('role','option');
    it.textContent = text;
    it.onclick = () => { addToken(text); };
    dropdown.appendChild(it);
  });
  dropdown.hidden = false;
}

/* --- Tokenization rules --- */
function addToken(text){
  const t = text.trim();
  if(!t) return;
  // If not exact ingredient, try to map to first suggestion
  let final = t;
  const matches = filterSuggestions(t);
  if(matches.length && matches[0].toLowerCase() !== t.toLowerCase()){
    final = matches[0];
  }
  uniquePush(tokens, final);
  input.value = '';
  activeIndex = -1;
  renderTokens();
  renderDropdown([]); // hide
}
function currentSuggestions(){
  return Array.from(dropdown.querySelectorAll('.item')).map(n=>n.textContent);
}

/* --- Keyboard interactions (FIXED ENTER LOGIC) --- */
input.addEventListener('keydown', (e)=>{
  const items = currentSuggestions();
  const commitKeys = ['Enter', 'Tab', ','];

  // Escape closes dropdown
  if (e.key === 'Escape') {
    dropdown.hidden = true; activeIndex = -1; return;
  }

  // Backspace deletes last token if input is empty
  if (e.key === 'Backspace' && input.value.trim() === '' && tokens.length) {
    e.preventDefault();
    tokens.pop(); renderTokens(); return;
  }

  // Up/Down navigates dropdown
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    const has = !dropdown.hidden && items.length > 0;
    if (!has) return;
    e.preventDefault();
    if (e.key === 'ArrowDown') activeIndex = (activeIndex + 1) % items.length;
    else activeIndex = (activeIndex - 1 + items.length) % items.length;
    renderDropdown(items); // re-render with active class
    return;
  }

  // ENTER / TAB / COMMA behavior
  if (commitKeys.includes(e.key)) {
    // Ctrl/⌘+Enter: commit (if any text) then search immediately
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      if (input.value.trim() !== '') {
        if (!dropdown.hidden && items.length && activeIndex >= 0) addToken(items[activeIndex]);
        else addToken(input.value);
      }
      if (tokens.length) suggest();
      return;
    }

    // If input is empty: ENTER triggers search
    if (e.key === 'Enter' && input.value.trim() === '') {
      e.preventDefault();
      if (tokens.length) suggest(); // search only when we have tokens
      return;
    }

    // Otherwise, commit the token (from dropdown selection if present)
    e.preventDefault();
    if (!dropdown.hidden && items.length && activeIndex >= 0) {
      addToken(items[activeIndex]);
    } else {
      addToken(input.value);
    }
  }
});

input.addEventListener('input', (e)=>{
  const q = e.target.value;
  const items = filterSuggestions(q);
  activeIndex = -1;
  renderDropdown(items);
});

// Also allow Enter to search when focus is on the token box and input is empty
tokenBox.addEventListener('keydown', (e)=>{
  if (e.key === 'Enter' && input.value.trim() === '' && tokens.length){
    e.preventDefault();
    suggest();
  }
});

document.addEventListener('click', (e)=>{
  if(!tokenBox.contains(e.target) && !dropdown.contains(e.target)){
    dropdown.hidden = true; activeIndex = -1;
  }
});

/* --- Quick chips (top 10) --- */
async function fetchIngredients(){
  try{
    const r = await fetch('/api/ingredients');
    if(!r.ok) return [];
    return await r.json();
  }catch{ return []; }
}
function renderChips(all){
  const elc = document.getElementById('chips'); elc.innerHTML = '';
  all.slice(0, 10).forEach(name=>{
    const d = document.createElement('div'); d.className='chip'; d.textContent=name;
    d.onclick = ()=>{ addToken(name); input.focus(); };
    elc.appendChild(d);
  });
}

/* --- Search trigger --- */
async function suggest(){
  if(tokens.length === 0) return;
  const have = encodeURIComponent(tokens.join(','));
  const r = await fetch('/api/suggest?have=' + have);
  const data = await r.json();

  document.getElementById('result').style.display = 'block';
  const mapped = (data.mapped||[]).join(', ') || '—';
  document.getElementById('mapped').textContent = 'Matched: ' + mapped;

  const unk = document.getElementById('unknown');
  if(data.unrecognized && data.unrecognized.length){
    unk.style.display='block';
    unk.textContent='Unrecognized: ' + data.unrecognized.join(', ');
  }else{
    unk.style.display='none';
  }

  const list = document.getElementById('list'); list.innerHTML='';
  (data.suggestions||[]).forEach(rec=>{
    const item = document.createElement('div'); item.className='cardItem';
    const t = document.createElement('div'); t.className='itemTitle';
    t.textContent = rec.inputs.join(' + ') + ' \u2192 ' + rec.output + ' (x' + rec.qty + ')';
    const m = document.createElement('div'); m.className='itemMeta';
    m.textContent = 'Inputs: ' + rec.inputs.join(', ');
    item.appendChild(t); item.appendChild(m); list.appendChild(item);
  });
}
suggestBtn.onclick = suggest;

/* Focus behavior */
tokenBox.addEventListener('click', ()=> input.focus());

/* Init */
fetchIngredients().then(arr => { ALL_ING = arr || []; renderChips(ALL_ING); });
renderTokens();
</script>
</body>
</html>`))

// ---------- Main ----------

func main() {
	var csvPath string
	var addr string
	flag.StringVar(&csvPath, "csv", "food.csv", "Path to food.csv (recipe table)")
	flag.StringVar(&addr, "addr", ":8080", "Listen address")
	flag.Parse()

	// Expand relative CSV path for logging clarity
	if !filepath.IsAbs(csvPath) {
		if abs, err := filepath.Abs(csvPath); err == nil {
			csvPath = abs
		}
	}
	db, err := loadCSV(csvPath)
	if err != nil {
		log.Fatalf("load csv: %v", err)
	}
	if len(db.Recipes) == 0 {
		log.Fatalf("no recipes parsed from %s", csvPath)
	}

	log.Printf("recipes: %d | ingredients: %d | csv: %s", len(db.Recipes), len(db.AllIngredients), csvPath)
	if err := serve(db, addr); err != nil {
		log.Fatal(err)
	}
}
