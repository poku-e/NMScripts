package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type apiResp struct {
	Mapped       []string `json:"mapped"`
	Unrecognized []string `json:"unrecognized"`
	Suggestions  []Recipe `json:"suggestions"`
}

type glyphCreateReq struct {
	Name        string `json:"name"`
	Symbols     string `json:"symbols"`
	Description string `json:"description"`
}

func serve(foodDB *DB, refDB *DB, gs *GlyphStore, addr string) error {
	mux := http.NewServeMux()

	imgDir := filepath.Join(filepath.Dir(gs.Path), "glyph-images")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return err
	}
	mux.Handle("/glyph-images/", http.StripPrefix("/glyph-images/", http.FileServer(http.Dir(imgDir))))

	// Recipes API
	mux.HandleFunc("/api/suggest", func(w http.ResponseWriter, r *http.Request) {
		have := strings.TrimSpace(r.URL.Query().Get("have"))
		if have == "" {
			http.Error(w, "missing 'have' query param", http.StatusBadRequest)
			return
		}
		parts := splitCSVLike(have)
		mapped, unknown := foodDB.mapUserIngredients(parts)
		if mapped == nil {
			mapped = []string{}
		}
		if unknown == nil {
			unknown = []string{}
		}
		sugs := foodDB.suggest(mapped)
		if sugs == nil {
			sugs = []Recipe{}
		}

		resp := apiResp{
			Mapped:       mapped,
			Unrecognized: unknown,
			Suggestions:  sugs,
		}
		writeJSON(w, resp)
	})

	mux.HandleFunc("/api/ingredients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, foodDB.AllIngredients)
	})

	// Refiner API
	mux.HandleFunc("/api/refiner/suggest", func(w http.ResponseWriter, r *http.Request) {
		have := strings.TrimSpace(r.URL.Query().Get("have"))
		if have == "" {
			http.Error(w, "missing 'have' query param", http.StatusBadRequest)
			return
		}
		parts := splitCSVLike(have)
		mapped, unknown := refDB.mapUserIngredients(parts)
		if mapped == nil {
			mapped = []string{}
		}
		if unknown == nil {
			unknown = []string{}
		}
		sugs := refDB.suggest(mapped)
		if sugs == nil {
			sugs = []Recipe{}
		}

		resp := apiResp{
			Mapped:       mapped,
			Unrecognized: unknown,
			Suggestions:  sugs,
		}
		writeJSON(w, resp)
	})

	mux.HandleFunc("/api/refiner/ingredients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, refDB.AllIngredients)
	})

	// Glyphs API
	mux.HandleFunc("/api/glyphs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, gs.List())
			return
		case http.MethodPost:
			ct := r.Header.Get("Content-Type")
			if strings.HasPrefix(ct, "multipart/form-data") {
				if err := r.ParseMultipartForm(10 << 20); err != nil {
					http.Error(w, "invalid form", http.StatusBadRequest)
					return
				}
				name := r.FormValue("name")
				symbols := r.FormValue("symbols")
				desc := r.FormValue("description")
				var photo []byte
				if file, _, err := r.FormFile("photo"); err == nil {
					defer file.Close()
					photo, err = io.ReadAll(io.LimitReader(file, 10<<20))
					if err != nil {
						http.Error(w, "invalid photo", http.StatusBadRequest)
						return
					}
				} else if err != http.ErrMissingFile {
					http.Error(w, "invalid photo", http.StatusBadRequest)
					return
				}
				g, err := gs.Add(name, symbols, desc, photo)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, g)
				return
			}
			var req glyphCreateReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			g, err := gs.Add(req.Name, req.Symbols, req.Description, nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, g)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	// Glyphs UI
	mux.HandleFunc("/glyphs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var buf bytes.Buffer
		if err := glyphsTmpl.Execute(&buf, nil); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "error writing response: %v\n", err)
			return
		}
	})

	// Refiner UI
	mux.HandleFunc("/refiner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var buf bytes.Buffer
		if err := refinerTmpl.Execute(&buf, nil); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			fmt.Fprintf(os.Stderr, "error writing response: %v\n", err)
			return
		}
	})

	// Recipe UI
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
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
