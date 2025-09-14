package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

func serve(db *DB, gs *GlyphStore, addr string) error {
	mux := http.NewServeMux()

	// Recipes API
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

	mux.HandleFunc("/api/ingredients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, db.AllIngredients)
	})

	// Glyphs API
	mux.HandleFunc("/api/glyphs", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, gs.List())
			return
		case http.MethodPost:
			var req glyphCreateReq
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			g, err := gs.Add(req.Name, req.Symbols, req.Description)
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
