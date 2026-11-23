// Package intentrules is the design-time rules layer: the intent model is written in Infrahub's schema language, the checks
// that a schema cannot say are declarative data, and a typed Go evaluator judges a netlab topology against both before any
// compile runs. Findings come out in the shape of internal/netlabref.Finding, so every surface that shows a compile's findings
// shows these too.
//
// Authority: netlab's compile result wins. This layer is earlier, not stronger: for any finding code netlab already owns, the
// compile's finding is kept and the static one is dropped (see MergeWithCompile).
package intentrules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Schema is the subset of an Infrahub schema file the evaluator reads, with inheritance resolved.
type Schema struct {
	Version string
	// Kinds by full kind name (namespace + name, for example NetlabDevice). Generics are included.
	Kinds map[string]*Kind
	// Order is the order kinds were declared in, for stable output.
	Order []string
}

// Kind is a node or a generic.
type Kind struct {
	Name          string // full kind: namespace + name
	Label         string
	Description   string
	Generic       bool
	InheritFrom   []string
	Attributes    []*Attribute    // own + inherited, in declaration order
	Relationships []*Relationship // own + inherited
	Uniqueness    []Uniqueness    // own and inherited
	// Concrete lists, for a generic, the nodes that inherit it.
	Concrete []string
}

// Uniqueness is one uniqueness_constraints entry. Elements are "attr__value" or a relationship name. Scope is the kind the
// constraint was declared on: a constraint declared on a generic holds across every node that inherits it (two links of different
// kinds may not share an id), not within each node on its own.
type Uniqueness struct {
	Elements []string
	Scope    string
}

// Attribute is one attribute of a kind.
type Attribute struct {
	Name        string
	Kind        string // Text, Number, Boolean, Dropdown, ...
	Optional    bool
	HasDefault  bool
	Default     any
	Regex       string
	Choices     []string // Dropdown choices, or a Text enum
	MinLength   *int
	MaxLength   *int
	MinValue    *int
	MaxValue    *int
	Description string
}

// Relationship is one relationship of a kind.
type Relationship struct {
	Name        string
	Peer        string
	Kind        string // Attribute, Component, Parent, ...
	Cardinality string // one | many
	Optional    bool
	MinCount    int
	MaxCount    int
	Identifier  string
	Description string
}

// ---------------------------------------------------------------------------------------------------------------------------
// The published Infrahub JSON schema names every key a schema file may carry (additionalProperties is false everywhere). The
// loader uses it to reject a key Infrahub would reject, so a key of ours slipped into the schema directory fails here, at load,
// and not only in the gate.

var (
	allowedOnce sync.Once
	allowedKeys map[string]map[string]bool // def -> key set
	allowedErr  error
)

func loadAllowedKeys() (map[string]map[string]bool, error) {
	allowedOnce.Do(func() {
		raw, err := embedded.ReadFile("jsonschema/infrahub-schema.json")
		if err != nil {
			allowedErr = err
			return
		}
		var doc struct {
			Props map[string]json.RawMessage `json:"properties"`
			Defs  map[string]struct {
				Props map[string]json.RawMessage `json:"properties"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			allowedErr = err
			return
		}
		keys := func(m map[string]json.RawMessage) map[string]bool {
			out := map[string]bool{}
			for k := range m {
				out[k] = true
			}
			return out
		}
		allowedKeys = map[string]map[string]bool{"root": keys(doc.Props)}
		union := func(name string, defs ...string) {
			set := map[string]bool{}
			for _, d := range defs {
				for k := range doc.Defs[d].Props {
					set[k] = true
				}
			}
			allowedKeys[name] = set
		}
		union("node", "NodeSchemaWrite")
		union("generic", "GenericSchemaWrite")
		union("relationship", "RelationshipSchemaWrite")
		union("attribute", "GenericAttributeWrite", "ListAttributeWrite", "NumberAttributeWrite", "NumberPoolAttributeWrite", "TextAttributeWrite")
		union("attribute.parameters", "AttributeParametersWrite", "ListAttributeParametersWrite", "NumberAttributeParametersWrite", "TextAttributeParametersWrite", "NumberPoolParametersWrite")
		for name, set := range allowedKeys {
			if len(set) == 0 {
				allowedErr = fmt.Errorf("the embedded Infrahub JSON schema has no keys for %q: it is not the published schema", name)
			}
		}
	})
	return allowedKeys, allowedErr
}

// LoadSchema parses one Infrahub schema file. It rejects, as Infrahub does, any key the published schema does not list.
func LoadSchema(name string, data []byte) (*Schema, error) {
	allowed, err := loadAllowedKeys()
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: a schema file is a YAML mapping", name)
	}
	if err := rejectUnknown(doc.Content[0], allowed["root"], name, "root"); err != nil {
		return nil, err
	}
	var raw struct {
		Version  string    `yaml:"version"`
		Generics []rawKind `yaml:"generics"`
		Nodes    []rawKind `yaml:"nodes"`
	}
	if err := doc.Content[0].Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if raw.Version == "" {
		return nil, fmt.Errorf("%s: version is required", name)
	}
	// key checks for every node, generic, attribute and relationship
	for _, section := range []struct{ key, kind string }{{"generics", "generic"}, {"nodes", "node"}} {
		seq := mapValue(doc.Content[0], section.key)
		if seq == nil {
			continue
		}
		for i, item := range seq.Content {
			where := fmt.Sprintf("%s[%d]", section.key, i)
			if err := rejectUnknown(item, allowed[section.kind], name, where); err != nil {
				return nil, err
			}
			if attrs := mapValue(item, "attributes"); attrs != nil {
				for j, a := range attrs.Content {
					if err := rejectUnknown(a, allowed["attribute"], name, fmt.Sprintf("%s.attributes[%d]", where, j)); err != nil {
						return nil, err
					}
					if p := mapValue(a, "parameters"); p != nil {
						if err := rejectUnknown(p, allowed["attribute.parameters"], name, fmt.Sprintf("%s.attributes[%d].parameters", where, j)); err != nil {
							return nil, err
						}
					}
				}
			}
			if rels := mapValue(item, "relationships"); rels != nil {
				for j, r := range rels.Content {
					if err := rejectUnknown(r, allowed["relationship"], name, fmt.Sprintf("%s.relationships[%d]", where, j)); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	s := &Schema{Version: raw.Version, Kinds: map[string]*Kind{}}
	add := func(r rawKind, generic bool) error {
		full := r.Namespace + r.Name
		if r.Name == "" || r.Namespace == "" {
			return fmt.Errorf("%s: a node needs a name and a namespace", name)
		}
		if _, dup := s.Kinds[full]; dup {
			return fmt.Errorf("%s: kind %s is declared twice", name, full)
		}
		k := &Kind{Name: full, Label: r.Label, Description: strings.TrimSpace(r.Description), Generic: generic}
		for _, u := range r.Uniqueness {
			k.Uniqueness = append(k.Uniqueness, Uniqueness{Elements: u, Scope: full})
		}
		for _, p := range r.InheritFrom {
			k.InheritFrom = append(k.InheritFrom, p)
		}
		for _, a := range r.Attributes {
			k.Attributes = append(k.Attributes, a.toAttribute())
		}
		for _, rel := range r.Relationships {
			k.Relationships = append(k.Relationships, rel.toRelationship())
		}
		s.Kinds[full] = k
		s.Order = append(s.Order, full)
		return nil
	}
	for _, g := range raw.Generics {
		if err := add(g, true); err != nil {
			return nil, err
		}
	}
	for _, n := range raw.Nodes {
		if err := add(n, false); err != nil {
			return nil, err
		}
	}
	if err := s.resolve(name); err != nil {
		return nil, err
	}
	return s, nil
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func rejectUnknown(n *yaml.Node, allowed map[string]bool, file, where string) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		if !allowed[k] {
			return fmt.Errorf("%s: %s: key %q is not an Infrahub schema key (Infrahub rejects it: extensions belong in the sidecar directory)", file, where, k)
		}
	}
	return nil
}

type rawKind struct {
	Name          string     `yaml:"name"`
	Namespace     string     `yaml:"namespace"`
	Label         string     `yaml:"label"`
	Description   string     `yaml:"description"`
	InheritFrom   []string   `yaml:"inherit_from"`
	Uniqueness    [][]string `yaml:"uniqueness_constraints"`
	Attributes    []rawAttr  `yaml:"attributes"`
	Relationships []rawRel   `yaml:"relationships"`
}

type rawAttr struct {
	Name         string `yaml:"name"`
	Kind         string `yaml:"kind"`
	Optional     *bool  `yaml:"optional"`
	DefaultValue any    `yaml:"default_value"`
	Regex        string `yaml:"regex"`
	Enum         []any  `yaml:"enum"`
	MinLength    *int   `yaml:"min_length"`
	MaxLength    *int   `yaml:"max_length"`
	Description  string `yaml:"description"`
	Choices      []struct {
		Name any `yaml:"name"`
	} `yaml:"choices"`
	Parameters struct {
		Regex     string `yaml:"regex"`
		MinLength *int   `yaml:"min_length"`
		MaxLength *int   `yaml:"max_length"`
		MinValue  *int   `yaml:"min_value"`
		MaxValue  *int   `yaml:"max_value"`
	} `yaml:"parameters"`
}

func (r rawAttr) toAttribute() *Attribute {
	a := &Attribute{Name: r.Name, Kind: r.Kind, Optional: r.Optional != nil && *r.Optional, Description: strings.TrimSpace(r.Description),
		Regex: r.Regex, MinLength: r.MinLength, MaxLength: r.MaxLength}
	if r.DefaultValue != nil {
		a.HasDefault = true
		a.Default = r.DefaultValue
	}
	if a.Regex == "" {
		a.Regex = r.Parameters.Regex
	}
	if a.MinLength == nil {
		a.MinLength = r.Parameters.MinLength
	}
	if a.MaxLength == nil {
		a.MaxLength = r.Parameters.MaxLength
	}
	a.MinValue, a.MaxValue = r.Parameters.MinValue, r.Parameters.MaxValue
	for _, c := range r.Choices {
		a.Choices = append(a.Choices, fmt.Sprint(c.Name))
	}
	for _, e := range r.Enum {
		a.Choices = append(a.Choices, fmt.Sprint(e))
	}
	return a
}

type rawRel struct {
	Name        string `yaml:"name"`
	Peer        string `yaml:"peer"`
	Kind        string `yaml:"kind"`
	Cardinality string `yaml:"cardinality"`
	Optional    *bool  `yaml:"optional"`
	MinCount    int    `yaml:"min_count"`
	MaxCount    int    `yaml:"max_count"`
	Identifier  string `yaml:"identifier"`
	Description string `yaml:"description"`
}

func (r rawRel) toRelationship() *Relationship {
	card := r.Cardinality
	if card == "" {
		card = "many"
	}
	// Infrahub's default is optional: true; a relationship is required only when the schema says optional: false.
	optional := r.Optional == nil || *r.Optional
	return &Relationship{Name: r.Name, Peer: r.Peer, Kind: r.Kind, Cardinality: card, Optional: optional,
		MinCount: r.MinCount, MaxCount: r.MaxCount, Identifier: r.Identifier, Description: strings.TrimSpace(r.Description)}
}

// resolve copies the attributes, relationships and uniqueness constraints of every inherited generic into the node, checks that
// every peer and inherit_from names a declared kind, and records which nodes implement each generic.
func (s *Schema) resolve(file string) error {
	for _, name := range s.Order {
		k := s.Kinds[name]
		for _, parent := range k.InheritFrom {
			g, ok := s.Kinds[parent]
			if !ok || !g.Generic {
				return fmt.Errorf("%s: %s inherits from %s, which is not a generic in this schema", file, name, parent)
			}
			k.Attributes = append(append([]*Attribute(nil), g.Attributes...), k.Attributes...)
			k.Relationships = append(append([]*Relationship(nil), g.Relationships...), k.Relationships...)
			k.Uniqueness = append(append([]Uniqueness(nil), g.Uniqueness...), k.Uniqueness...)
			g.Concrete = append(g.Concrete, name)
		}
	}
	for _, name := range s.Order {
		k := s.Kinds[name]
		seen := map[string]bool{}
		for _, a := range k.Attributes {
			if seen[a.Name] {
				return fmt.Errorf("%s: %s declares %q twice", file, name, a.Name)
			}
			seen[a.Name] = true
		}
		for _, r := range k.Relationships {
			if seen[r.Name] {
				return fmt.Errorf("%s: %s declares %q twice", file, name, r.Name)
			}
			seen[r.Name] = true
			if _, ok := s.Kinds[r.Peer]; !ok {
				return fmt.Errorf("%s: %s.%s names the peer %s, which is not declared", file, name, r.Name, r.Peer)
			}
		}
		for _, u := range k.Uniqueness {
			for _, el := range u.Elements {
				if !seen[strings.TrimSuffix(el, "__value")] {
					return fmt.Errorf("%s: %s uniqueness constraint names %q, which is not a member", file, name, el)
				}
			}
		}
	}
	return nil
}

// Attr returns the attribute of a kind by name.
func (k *Kind) Attr(name string) *Attribute {
	for _, a := range k.Attributes {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Rel returns the relationship of a kind by name.
func (k *Kind) Rel(name string) *Relationship {
	for _, r := range k.Relationships {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// SortedKinds returns the kind names in declaration order, generics first.
func (s *Schema) SortedKinds() []string { return append([]string(nil), s.Order...) }

// isA reports whether an object of kind `have` satisfies a relationship peer / iteration kind `want` (the kind itself, or a
// generic it inherits).
func (s *Schema) isA(have, want string) bool {
	if have == want {
		return true
	}
	if k, ok := s.Kinds[have]; ok {
		for _, p := range k.InheritFrom {
			if p == want {
				return true
			}
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// strictDecode decodes YAML into v and rejects unknown fields.
func strictDecode(data []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(v)
}
