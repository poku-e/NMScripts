// scrape_nms_table.go
//
// Usage examples:
//
//	go run ./scrape_nms_table.go --url "https://app.nmsassistant.com/cooking" --out out.csv
//	go run ./scrape_nms_table.go --url "https://app.nmsassistant.com/cooking" --out out.xlsx
//	go run ./scrape_nms_table.go --url "https://app.nmsassistant.com/cooking" --out out.csv --selector "#table"
//
// go.mod (minimal):
//
//	module example.com/scrape
//	go 1.21
//	require (
//	  github.com/PuerkitoBio/goquery v1.9.2
//	  github.com/xuri/excelize/v2 v2.9.0
//	)
//
// Get deps:
//
//	go get github.com/PuerkitoBio/goquery@latest
//	go get github.com/xuri/excelize/v2@latest
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/xuri/excelize/v2"
)

type Cell struct {
	Name string
	Qty  *int
	Href string
	Img  string
	Bg   string
}

type Row struct {
	Input1 Cell
	Input2 Cell
	Input3 Cell
	Output Cell
}

var (
	amountRe = regexp.MustCompile(`(?i)\bx\s*(\d+)\b`)
	bgRe     = regexp.MustCompile(`(?i)background:\s*([^;]+)`)
	spaceRe  = regexp.MustCompile(`\s+`)
)

// ---------- HTTP with retry ----------
func httpClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		// Reasonable defaults; keepalives enabled
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func fetch(ctx context.Context, rawURL string) (html string, finalBase *url.URL, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := httpClient(25 * time.Second)

	var resp *http.Response
	// Simple bounded retry on transient status codes/timeouts.
	backoffs := []time.Duration{0, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	for i, d := range backoffs {
		if d > 0 {
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return "", nil, ctx.Err()
			}
		}
		resp, err = client.Do(req)
		if err != nil {
			// retry on network errors
			if i < len(backoffs)-1 {
				continue
			}
			return "", nil, err
		}
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			_ = resp.Body.Close()
			if i < len(backoffs)-1 {
				continue
			}
			return "", nil, fmt.Errorf("server error: %s", resp.Status)
		}
		break
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", nil, fmt.Errorf("bad status %d: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	u, err := url.Parse(resp.Request.URL.String())
	if err != nil {
		return "", nil, err
	}
	return string(b), u, nil
}

// ---------- Parsing ----------
func parseQtyFromText(s string) *int {
	if s == "" {
		return nil
	}
	m := amountRe.FindStringSubmatch(s)
	if len(m) == 2 {
		val := atoiSafe(m[1])
		return &val
	}
	return nil
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			continue
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func parseBG(style string) string {
	if style == "" {
		return ""
	}
	m := bgRe.FindStringSubmatch(style)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func textCondense(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func resolve(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(ru).String()
}

func first(sel *goquery.Selection) string {
	if sel.Length() == 0 {
		return ""
	}
	return textCondense(sel.First().Text())
}

func extractCell(td *goquery.Selection, base *url.URL) Cell {
	if td == nil || td.Length() == 0 {
		return Cell{}
	}

	// 1) Preferred name: hidden <span class="... sort ...">
	name := first(td.Find("span.sort"))
	if name == "" {
		// 2) Visible .cell-text minus any trailing "xN"
		vis := first(td.Find(".cell-text"))
		if vis != "" {
			name = strings.TrimSpace(amountRe.ReplaceAllString(vis, ""))
			if name == "" {
				name = vis // fallback if replace made empty
			}
		}
	}
	if name == "" {
		// 3) Fallback to <img alt=...>
		if img := td.Find("img"); img.Length() != 0 {
			if alt, ok := img.Attr("alt"); ok {
				name = strings.TrimSpace(alt)
			}
		}
	}

	// qty from <span class="amount"> or any xN fragment
	var qty *int
	if amt := first(td.Find("span.amount")); amt != "" {
		qty = parseQtyFromText(amt)
	}
	if qty == nil {
		// Sometimes amount is only in the visible text
		vis := first(td.Find(".cell-text"))
		qty = parseQtyFromText(vis)
	}
	if qty == nil && name != "" {
		// default to 1 when a name exists but no explicit qty
		one := 1
		qty = &one
	}

	// href absolute
	var href string
	if a := td.Find("a").First(); a.Length() != 0 {
		if h, ok := a.Attr("href"); ok {
			href = resolve(base, h)
		}
	}

	// img absolute
	var imgURL string
	if img := td.Find("img").First(); img.Length() != 0 {
		if s, ok := img.Attr("src"); ok {
			imgURL = resolve(base, s)
		}
	}

	// background from .cell-content style
	var bg string
	if div := td.Find("div.cell-content").First(); div.Length() != 0 {
		if style, ok := div.Attr("style"); ok {
			bg = parseBG(style)
		}
	}

	return Cell{
		Name: name,
		Qty:  qty,
		Href: href,
		Img:  imgURL,
		Bg:   bg,
	}
}

func parseTable(html string, base *url.URL, selector string) ([]Row, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	table := doc.Find(selector).First()
	if table.Length() == 0 {
		return nil, fmt.Errorf("table not found with selector %q", selector)
	}

	var out []Row
	table.Find("tbody > tr").Each(func(_ int, tr *goquery.Selection) {
		tds := tr.Find("td")
		getTD := func(i int) *goquery.Selection {
			if i < 0 || i >= tds.Length() {
				return nil
			}
			return tds.Eq(i)
		}
		row := Row{
			Input1: extractCell(getTD(0), base),
			Input2: extractCell(getTD(1), base),
			Input3: extractCell(getTD(2), base),
			Output: extractCell(getTD(3), base),
		}
		out = append(out, row)
	})
	return out, nil
}

// ---------- Output writers ----------
func writeCSV(path string, rows []Row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"input1_name", "input1_qty", "input1_href", "input1_img", "input1_bg",
		"input2_name", "input2_qty", "input2_href", "input2_img", "input2_bg",
		"input3_name", "input3_qty", "input3_href", "input3_img", "input3_bg",
		"output_name", "output_qty", "output_href", "output_img", "output_bg",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		rec := []string{
			r.Input1.Name, qtyStr(r.Input1.Qty), r.Input1.Href, r.Input1.Img, r.Input1.Bg,
			r.Input2.Name, qtyStr(r.Input2.Qty), r.Input2.Href, r.Input2.Img, r.Input2.Bg,
			r.Input3.Name, qtyStr(r.Input3.Qty), r.Input3.Href, r.Input3.Img, r.Input3.Bg,
			r.Output.Name, qtyStr(r.Output.Qty), r.Output.Href, r.Output.Img, r.Output.Bg,
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return w.Error()
}

func writeXLSX(path string, rows []Row) error {
	f := excelize.NewFile()
	const sheet = "Sheet1"
	// StreamWriter for efficiency on large tables
	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}
	header := []interface{}{
		"input1_name", "input1_qty", "input1_href", "input1_img", "input1_bg",
		"input2_name", "input2_qty", "input2_href", "input2_img", "input2_bg",
		"input3_name", "input3_qty", "input3_href", "input3_img", "input3_bg",
		"output_name", "output_qty", "output_href", "output_img", "output_bg",
	}
	if err := sw.SetRow("A1", header); err != nil {
		return err
	}
	for i, r := range rows {
		row := []interface{}{
			r.Input1.Name, qtyStr(r.Input1.Qty), r.Input1.Href, r.Input1.Img, r.Input1.Bg,
			r.Input2.Name, qtyStr(r.Input2.Qty), r.Input2.Href, r.Input2.Img, r.Input2.Bg,
			r.Input3.Name, qtyStr(r.Input3.Qty), r.Input3.Href, r.Input3.Img, r.Input3.Bg,
			r.Output.Name, qtyStr(r.Output.Qty), r.Output.Href, r.Output.Img, r.Output.Bg,
		}
		cellAddr, _ := excelize.CoordinatesToCellName(1, i+2) // A2, A3, ...
		if err := sw.SetRow(cellAddr, row); err != nil {
			return err
		}
	}
	if err := sw.Flush(); err != nil {
		return err
	}
	return f.SaveAs(path)
}

func qtyStr(q *int) string {
	if q == nil {
		return ""
	}
	return fmt.Sprintf("%d", *q)
}

// ---------- Main ----------
func main() {
	var (
		pageURL  string
		outPath  string
		selector string
	)
	flag.StringVar(&pageURL, "url", "", "Page URL to fetch (required)")
	flag.StringVar(&outPath, "out", "", "Output file path (.csv or .xlsx) (required)")
	flag.StringVar(&selector, "selector", "#table", "CSS selector for the target table")
	flag.Parse()

	if pageURL == "" || outPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	html, base, err := fetch(ctx, pageURL)
	if err != nil {
		fatal(err)
	}
	rows, err := parseTable(html, base, selector)
	if err != nil {
		fatal(err)
	}
	if len(rows) == 0 {
		fatal(errors.New("parsed 0 rows; check selector or that the page is server-rendered"))
	}

	switch {
	case strings.HasSuffix(strings.ToLower(outPath), ".csv"):
		err = writeCSV(outPath, rows)
	case strings.HasSuffix(strings.ToLower(outPath), ".xlsx"):
		err = writeXLSX(outPath, rows)
	default:
		fatal(errors.New("out must end with .csv or .xlsx"))
	}
	if err != nil {
		fatal(err)
	}

	fmt.Printf("OK: %d rows -> %s\n", len(rows), outPath)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(1)
}
