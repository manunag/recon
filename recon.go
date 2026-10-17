// Package main implements recon, a small tool that reconciles two JSONL
// datasets and groups every difference into a category, so a large
// mismatch list becomes a short list of fixable problems.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// Record is one JSON object keyed by its identifier.
type Record map[string]any

// Category names a kind of mismatch, for example "missing_in_target" or "field:email".
type Category string

const (
	MissingInTarget Category = "missing_in_target"
	ExtraInTarget   Category = "extra_in_target"
)

// Bucket holds the count and a bounded sample of IDs for one category.
type Bucket struct {
	Count   int      `json:"count"`
	Samples []string `json:"samples"`
}

// Report is the result of a reconciliation.
type Report struct {
	SourceCount int                  `json:"source_count"`
	TargetCount int                  `json:"target_count"`
	Matched     int                  `json:"matched"`
	Categories  map[Category]*Bucket `json:"categories"`
}

// Options controls a reconciliation.
type Options struct {
	Key     string          // field that identifies a record
	Ignore  map[string]bool // fields to leave out of comparison
	Samples int             // max sample IDs kept per category
}

// Load reads a JSONL stream into a map keyed by the record identifier.
// Blank lines are skipped; a duplicate key or a missing key is an error.
func Load(r io.Reader, key string) (map[string]Record, error) {
	out := make(map[string]Record)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(text), &rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		id, ok := rec[key]
		if !ok {
			return nil, fmt.Errorf("line %d: missing key %q", line, key)
		}
		k := fmt.Sprint(id)
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("line %d: duplicate key %q", line, k)
		}
		out[k] = rec
	}
	return out, sc.Err()
}

// LoadFiles reads the source and target files concurrently.
func LoadFiles(srcPath, dstPath, key string) (src, dst map[string]Record, err error) {
	var wg sync.WaitGroup
	var srcErr, dstErr error
	load := func(path string, out *map[string]Record, errp *error) {
		defer wg.Done()
		f, e := os.Open(path)
		if e != nil {
			*errp = e
			return
		}
		defer f.Close()
		*out, *errp = Load(f, key)
		if *errp != nil {
			*errp = fmt.Errorf("%s: %w", path, *errp)
		}
	}
	wg.Add(2)
	go load(srcPath, &src, &srcErr)
	go load(dstPath, &dst, &dstErr)
	wg.Wait()
	if srcErr != nil {
		return nil, nil, srcErr
	}
	return src, dst, dstErr
}

// Compare reconciles src against dst. Output is deterministic: IDs are
// visited in sorted order, so samples are stable between runs.
func Compare(src, dst map[string]Record, opt Options) *Report {
	rep := &Report{
		SourceCount: len(src),
		TargetCount: len(dst),
		Categories:  make(map[Category]*Bucket),
	}
	add := func(c Category, id string) {
		b := rep.Categories[c]
		if b == nil {
			b = &Bucket{}
			rep.Categories[c] = b
		}
		b.Count++
		if len(b.Samples) < opt.Samples {
			b.Samples = append(b.Samples, id)
		}
	}

	for _, id := range sortedKeys(src) {
		d, ok := dst[id]
		if !ok {
			add(MissingInTarget, id)
			continue
		}
		diffs := diffFields(src[id], d, opt)
		if len(diffs) == 0 {
			rep.Matched++
			continue
		}
		for _, f := range diffs {
			add(Category("field:"+f), id)
		}
	}
	for _, id := range sortedKeys(dst) {
		if _, ok := src[id]; !ok {
			add(ExtraInTarget, id)
		}
	}
	return rep
}

// diffFields returns the sorted names of fields whose values differ.
func diffFields(a, b Record, opt Options) []string {
	seen := make(map[string]bool)
	var out []string
	check := func(f string) {
		if seen[f] || f == opt.Key || opt.Ignore[f] {
			return
		}
		seen[f] = true
		av, aok := a[f]
		bv, bok := b[f]
		if aok != bok || !reflect.DeepEqual(av, bv) {
			out = append(out, f)
		}
	}
	for f := range a {
		check(f)
	}
	for f := range b {
		check(f)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]Record) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Mismatches is the number of category entries (a record can count in several).
func (r *Report) Mismatches() int {
	n := 0
	for _, b := range r.Categories {
		n += b.Count
	}
	return n
}

// WriteText prints a short human-readable summary, largest category first.
func (r *Report) WriteText(w io.Writer) {
	fmt.Fprintf(w, "source: %d  target: %d  matched: %d\n", r.SourceCount, r.TargetCount, r.Matched)
	if len(r.Categories) == 0 {
		fmt.Fprintln(w, "no differences")
		return
	}
	cats := make([]Category, 0, len(r.Categories))
	for c := range r.Categories {
		cats = append(cats, c)
	}
	sort.Slice(cats, func(i, j int) bool {
		bi, bj := r.Categories[cats[i]], r.Categories[cats[j]]
		if bi.Count != bj.Count {
			return bi.Count > bj.Count
		}
		return cats[i] < cats[j]
	})
	for _, c := range cats {
		b := r.Categories[c]
		fmt.Fprintf(w, "  %-24s %6d   e.g. %s\n", c, b.Count, strings.Join(b.Samples, ", "))
	}
}
