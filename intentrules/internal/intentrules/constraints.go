package intentrules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Violation is one schema constraint a graph breaks.
type Violation struct {
	Object *Object
	// Member is the attribute or relationship the constraint is about (the first attribute of a uniqueness constraint).
	Member     string
	Constraint string // unique | ref | required | min_count | max_count | cardinality | regex | enum | range
	Count      int
	Detail     string
}

// Constraints returns every constraint of the schema that the graph breaks. It enforces what Infrahub enforces on write
// (uniqueness, peer existence, cardinality, max_count, required, regex, enum, length and range) and what Infrahub 1.11.11 does
// not: min_count, which is why a link with one end can be stored there and is caught here.
func (s *Schema) Constraints(g *Graph) []Violation {
	var out []Violation
	for _, o := range g.Objects {
		k, ok := s.Kinds[o.Kind]
		if !ok {
			continue
		}
		out = append(out, s.attributeViolations(k, o)...)
		out = append(out, s.relationshipViolations(g, k, o)...)
	}
	out = append(out, s.uniquenessViolations(g)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Object.Key < out[j].Object.Key })
	return out
}

// value returns the effective value of an attribute: the set value, else the schema default, else nil.
func value(k *Kind, o *Object, name string) any {
	if v, ok := o.Attrs[name]; ok && v != nil {
		return v
	}
	if a := k.Attr(name); a != nil && a.HasDefault {
		return a.Default
	}
	return nil
}

func (s *Schema) attributeViolations(k *Kind, o *Object) []Violation {
	var out []Violation
	for _, a := range k.Attributes {
		v := value(k, o, a.Name)
		if v == nil {
			if !a.Optional {
				out = append(out, Violation{Object: o, Member: a.Name, Constraint: "required", Detail: fmt.Sprintf("%s is required", a.Name)})
			}
			continue
		}
		text := str(v)
		if a.Regex != "" {
			if re, err := regexp.Compile(a.Regex); err == nil && !re.MatchString(text) {
				out = append(out, Violation{Object: o, Member: a.Name, Constraint: "regex", Detail: fmt.Sprintf("%s %q does not match %s", a.Name, text, a.Regex)})
			}
		}
		if len(a.Choices) > 0 {
			ok := false
			for _, c := range a.Choices {
				if c == text {
					ok = true
				}
			}
			if !ok {
				out = append(out, Violation{Object: o, Member: a.Name, Constraint: "enum", Detail: fmt.Sprintf("%s %q is not one of %s", a.Name, text, strings.Join(a.Choices, ", "))})
			}
		}
		if (a.MinLength != nil && len(text) < *a.MinLength) || (a.MaxLength != nil && len(text) > *a.MaxLength) {
			out = append(out, Violation{Object: o, Member: a.Name, Constraint: "range", Detail: fmt.Sprintf("%s has %d characters, outside the allowed length", a.Name, len(text))})
		}
		if a.MinValue != nil || a.MaxValue != nil {
			if n, err := strconv.Atoi(text); err == nil {
				if (a.MinValue != nil && n < *a.MinValue) || (a.MaxValue != nil && n > *a.MaxValue) {
					out = append(out, Violation{Object: o, Member: a.Name, Constraint: "range", Detail: fmt.Sprintf("%s %d is outside the allowed range", a.Name, n)})
				}
			}
		}
	}
	return out
}

func (s *Schema) relationshipViolations(g *Graph, k *Kind, o *Object) []Violation {
	var out []Violation
	for _, r := range k.Relationships {
		refs := o.Rels[r.Name]
		for _, ref := range refs {
			target := g.Get(ref.Key)
			if target == nil || !s.isA(target.Kind, r.Peer) {
				out = append(out, Violation{Object: o, Member: r.Name, Constraint: "ref", Detail: fmt.Sprintf("%s refers to %q, which does not exist", r.Name, ref.Name)})
			}
		}
		n := len(refs)
		if r.Cardinality == "one" && n > 1 {
			out = append(out, Violation{Object: o, Member: r.Name, Constraint: "cardinality", Count: n, Detail: fmt.Sprintf("%s allows one peer, found %d", r.Name, n)})
		}
		switch {
		case r.MinCount > 0 && n < r.MinCount:
			out = append(out, Violation{Object: o, Member: r.Name, Constraint: "min_count", Count: n, Detail: fmt.Sprintf("%s needs at least %d, found %d", r.Name, r.MinCount, n)})
		case !r.Optional && n == 0:
			out = append(out, Violation{Object: o, Member: r.Name, Constraint: "required", Count: 0, Detail: fmt.Sprintf("%s is required", r.Name)})
		}
		if r.MaxCount > 0 && n > r.MaxCount {
			out = append(out, Violation{Object: o, Member: r.Name, Constraint: "max_count", Count: n, Detail: fmt.Sprintf("%s allows at most %d, found %d", r.Name, r.MaxCount, n)})
		}
	}
	return out
}

func (s *Schema) uniquenessViolations(g *Graph) []Violation {
	type spec struct {
		scope    string
		elements []string
	}
	seen := map[string]spec{}
	for _, name := range s.Order {
		for _, u := range s.Kinds[name].Uniqueness {
			seen[u.Scope+"|"+strings.Join(u.Elements, ",")] = spec{u.Scope, u.Elements}
		}
	}
	var out []Violation
	for _, id := range sortedKeys(seen) {
		sp := seen[id]
		groups := map[string][]*Object{}
		var order []string
		for _, o := range g.Objects {
			if !s.isA(o.Kind, sp.scope) {
				continue
			}
			k := s.Kinds[o.Kind]
			parts := make([]string, 0, len(sp.elements))
			complete := true
			for _, el := range sp.elements {
				if name, isAttr := strings.CutSuffix(el, "__value"); isAttr {
					v := value(k, o, name)
					if v == nil || str(v) == "" {
						complete = false
						break
					}
					parts = append(parts, str(v))
					continue
				}
				keys := make([]string, 0, len(o.Rels[el]))
				for _, ref := range o.Rels[el] {
					keys = append(keys, ref.Key+"/"+ref.Name)
				}
				if len(keys) == 0 {
					complete = false
					break
				}
				sort.Strings(keys)
				parts = append(parts, strings.Join(keys, "+"))
			}
			if !complete {
				continue
			}
			key := strings.Join(parts, "\x00")
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], o)
		}
		member := sp.elements[0]
		for _, el := range sp.elements {
			if name, isAttr := strings.CutSuffix(el, "__value"); isAttr {
				member = name
				break
			}
		}
		for _, key := range order {
			objs := groups[key]
			for _, dup := range objs[1:] {
				out = append(out, Violation{Object: dup, Member: member, Constraint: "unique", Count: len(objs),
					Detail: fmt.Sprintf("%s is shared by %d objects", member, len(objs))})
			}
		}
	}
	return out
}
