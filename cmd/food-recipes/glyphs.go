package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ---------- Data model: Glyphs ----------

type Glyph struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Symbols     string    `json:"symbols"`     // raw glyph string
	Description string    `json:"description"` // free text
	Photo       string    `json:"photo,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type GlyphStore struct {
	mu    sync.RWMutex
	Path  string
	Items []Glyph
}

func (gs *GlyphStore) Load() error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if gs.Path == "" {
		return errors.New("glyph store path empty")
	}
	b, err := os.ReadFile(gs.Path)
	if err != nil {
		if os.IsNotExist(err) {
			gs.Items = nil
			return nil
		}
		return err
	}
	var items []Glyph
	if err := json.Unmarshal(b, &items); err != nil {
		return err
	}
	gs.Items = items
	return nil
}

func (gs *GlyphStore) Save() error {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	tmp := gs.Path + ".tmp"
	data, err := json.MarshalIndent(gs.Items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, gs.Path)
}

func (gs *GlyphStore) List() []Glyph {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	out := make([]Glyph, len(gs.Items))
	copy(out, gs.Items)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (gs *GlyphStore) Add(name, symbols, desc string, photo []byte) (Glyph, error) {
	name = strings.TrimSpace(name)
	symbols = strings.TrimSpace(symbols)
	desc = strings.TrimSpace(desc)

	if name == "" {
		return Glyph{}, errors.New("name required")
	}
	if symbols == "" {
		return Glyph{}, errors.New("symbols required")
	}
	if utf8.RuneCountInString(name) > 64 {
		return Glyph{}, errors.New("name too long (max 64 chars)")
	}
	if utf8.RuneCountInString(symbols) > 128 {
		return Glyph{}, errors.New("symbols too long (max 128 chars)")
	}
	if utf8.RuneCountInString(desc) > 512 {
		return Glyph{}, errors.New("description too long (max 512 chars)")
	}

	g := Glyph{
		ID:          fmt.Sprintf("%d_%x", time.Now().UnixNano(), xxhash(normKey(name+symbols))),
		Name:        name,
		Symbols:     symbols,
		Description: desc,
		CreatedAt:   time.Now().UTC(),
	}

	if len(photo) > 0 {
		img, _, err := image.Decode(bytes.NewReader(photo))
		if err != nil {
			return Glyph{}, fmt.Errorf("invalid photo: %w", err)
		}
		imgDir := filepath.Join(filepath.Dir(gs.Path), "glyph-images")
		if err := os.MkdirAll(imgDir, 0o755); err != nil {
			return Glyph{}, err
		}
		fp := filepath.Join(imgDir, g.ID+".jpg")
		f, err := os.Create(fp)
		if err != nil {
			return Glyph{}, err
		}
		if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
			f.Close()
			return Glyph{}, err
		}
		if err := f.Close(); err != nil {
			return Glyph{}, err
		}
		g.Photo = "/glyph-images/" + g.ID + ".jpg"
	}

	gs.mu.Lock()
	defer gs.mu.Unlock()

	for _, it := range gs.Items {
		if strings.EqualFold(it.Name, g.Name) && normKey(it.Symbols) == normKey(g.Symbols) {
			return Glyph{}, errors.New("duplicate glyph (same name & symbols)")
		}
	}
	gs.Items = append(gs.Items, g)

	tmp := gs.Path + ".tmp"
	data, err := json.MarshalIndent(gs.Items, "", "  ")
	if err != nil {
		return Glyph{}, err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return Glyph{}, err
	}
	if err := os.Rename(tmp, gs.Path); err != nil {
		return Glyph{}, err
	}
	return g, nil
}

// tiny non-crypto hash for IDs (FNV-1a 64)
func xxhash(s string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}
