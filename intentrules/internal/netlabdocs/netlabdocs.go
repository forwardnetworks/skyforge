// Package netlabdocs serves netlab's own documentation, embedded in the binary, to
// tools that author topologies (list, read and ranked search over the pages).
//
// The corpus is netlab's docs/ tree, generated from the netlab release the rest of
// the module is built against, so it describes the netlab that is in use and cannot
// drift from it. An index (path, size, SHA-256 per page) ships beside it and the
// tests check the two agree.
package netlabdocs

import (
	"encoding/json"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

type Entry struct {
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type index struct {
	Source  string  `json:"source"`
	Entries []Entry `json:"entries"`
}

var (
	once   sync.Once
	loaded index
)

func load() index {
	once.Do(func() {
		if err := json.Unmarshal(indexJSON, &loaded); err != nil {
			// The index ships beside the corpus and both are generated together, so
			// a parse failure is a build defect, not a runtime condition.
			panic("netlabdocs: embedded index failed to parse: " + err.Error())
		}
	})
	return loaded
}

// List returns every document, smallest path first.
func List() []Entry { return load().Entries }

// Read returns one document by its corpus-relative path (e.g. "module/eigrp.md").
func Read(path string) (string, bool) {
	path = strings.TrimPrefix(strings.TrimSpace(path), "/")
	if path == "" || strings.Contains(path, "..") {
		return "", false
	}
	body, err := corpusFS.ReadFile(corpusRoot + "/" + path)
	if err != nil {
		return "", false
	}
	return string(body), true
}

// Search returns the paths whose CONTENT or path mentions every term, RANKED, so
// a caller can find the right reference instead of being told what to think.
//
// RANKING IS NOT A NICETY. "link attribute" AND-ed matches 20+ docs because both
// words are common in a networking corpus; returned alphabetically, the answer
// (links.md) sits below addressing.md and caveats.md and the reader has to guess.
// A search that technically contains the answer but buries it is the same
// failure as no search at all.
//
// Score, highest first: a term in the FILENAME is worth far more than a term in
// the body, because netlab names its reference pages after their subject; then
// total occurrences; then a shorter path, which favours top-level references
// over deep examples. Ties break alphabetically so the order is stable.
func Search(terms ...string) []string {
	var want []string
	for _, t := range terms {
		if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
			want = append(want, t)
		}
	}
	if len(want) == 0 {
		return nil
	}
	type scored struct {
		path  string
		score int
	}
	var hits []scored
	_ = fs.WalkDir(corpusFS, corpusRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(p, ".md") || strings.HasSuffix(p, ".txt")) {
			return nil
		}
		body, rerr := corpusFS.ReadFile(p)
		if rerr != nil {
			return nil
		}
		rel := strings.TrimPrefix(p, corpusRoot+"/")
		name := strings.ToLower(rel)
		text := strings.ToLower(string(body))
		score := 0
		for _, t := range want {
			inName := strings.Contains(name, t)
			n := strings.Count(text, t)
			if !inName && n == 0 {
				return nil // AND: every term must appear somewhere
			}
			if inName {
				score += 1000
			}
			score += n
		}
		score -= len(rel) // prefer the shorter, more canonical path
		hits = append(hits, scored{rel, score})
		return nil
	})
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].path < hits[j].path
	})
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.path)
	}
	return out
}
