package intentrules

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Sidecar is Skyforge's extension of a schema: everything Infrahub would reject inside the schema file itself.
type Sidecar struct {
	Annotations map[string]*Annotation // keyed Kind.member
	Findings    map[string]FindingText // codes only this layer emits
}

// Annotation is what the sidecar says about one attribute or relationship.
type Annotation struct {
	Netlab   map[string]any    `yaml:"netlab"`
	Findings map[string]string `yaml:"findings"` // constraint -> finding code
	Messages map[string]string `yaml:"messages"` // constraint -> message template
}

// FindingText is the class, why and fix of a code that netlab's classifier does not own.
type FindingText struct {
	Class string `yaml:"class"`
	Why   string `yaml:"why"`
	Fix   string `yaml:"fix"`
}

// LoadSidecar parses a sidecar file and refuses any key that does not resolve to an attribute or relationship of the schema.
func LoadSidecar(name string, data []byte, s *Schema) (*Sidecar, error) {
	var raw struct {
		Version     int                    `yaml:"sidecar_version"`
		Annotations map[string]*Annotation `yaml:"annotations"`
		Findings    map[string]FindingText `yaml:"findings"`
	}
	if err := strictDecode(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if raw.Version != 1 {
		return nil, fmt.Errorf("%s: sidecar_version must be 1", name)
	}
	known := map[string]bool{"unique": true, "ref": true, "required": true, "min_count": true, "max_count": true, "cardinality": true,
		"regex": true, "enum": true, "range": true}
	for key, a := range raw.Annotations {
		kind, member, ok := strings.Cut(key, ".")
		if !ok {
			return nil, fmt.Errorf("%s: annotation key %q is not Kind.member", name, key)
		}
		k, ok := s.Kinds[kind]
		if !ok {
			return nil, fmt.Errorf("%s: annotation %q names the kind %s, which the schema does not declare", name, key, kind)
		}
		if k.Attr(member) == nil && k.Rel(member) == nil {
			return nil, fmt.Errorf("%s: annotation %q names %s, which %s does not have (the sidecar has drifted from the schema)", name, key, member, kind)
		}
		if a == nil {
			continue
		}
		for c := range a.Findings {
			if !known[c] {
				return nil, fmt.Errorf("%s: annotation %q: %q is not a constraint (%s)", name, key, c, strings.Join(sortedKeys(known), ", "))
			}
		}
		for c := range a.Messages {
			if !known[c] {
				return nil, fmt.Errorf("%s: annotation %q: message for %q, which is not a constraint", name, key, c)
			}
		}
	}
	for code, f := range raw.Findings {
		if f.Class != "will-not-work" && f.Class != "likely-wrong" && f.Class != "informational" {
			return nil, fmt.Errorf("%s: finding %s: class %q is not will-not-work, likely-wrong or informational", name, code, f.Class)
		}
		if f.Why == "" || (f.Class != "informational" && f.Fix == "") {
			return nil, fmt.Errorf("%s: finding %s needs a why and a fix", name, code)
		}
	}
	return &Sidecar{Annotations: raw.Annotations, Findings: raw.Findings}, nil
}

func (sc *Sidecar) annotation(kind, member string) *Annotation {
	if sc == nil {
		return nil
	}
	return sc.Annotations[kind+"."+member]
}

// embeddedFiles lists the embedded files under dir with the given suffix, sorted.
func embeddedFiles(dir, suffix string) ([]string, error) {
	entries, err := fs.ReadDir(embedded, dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			out = append(out, dir+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}
