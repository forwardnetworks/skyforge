package intentrules

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Pred is the predicate language of a rule: a small YAML AST, no scripting. A node is exactly one of
//
//	{attr: NAME, eq|ne|in|regex|exists: VALUE}   a test on an attribute of the object (the default counts)
//	{all: [PRED...]} {any: [PRED...]} {not: PRED}
//	{related: {rel: NAME, where: PRED, count_ge: N, count_le: M}}   a test on the objects a relationship points to
//
// A related node holds when the number of peers satisfying where is within [count_ge, count_le]; count_ge defaults to 1 unless only
// count_le is given (then 0).
type Pred struct {
	Attr    string
	Op      string // eq ne in regex exists
	Value   any
	Values  []any
	re      *regexp.Regexp
	All     []*Pred
	Any     []*Pred
	Not     *Pred
	Related *Related
}

// Related is the relationship test of a predicate.
type Related struct {
	Rel     string
	Where   *Pred
	CountGE *int
	CountLE *int
}

var (
	_ yaml.Unmarshaler = (*Pred)(nil)
)

// UnmarshalYAML enforces the shape: exactly one form per node, and nothing the language does not define.
func (p *Pred) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a predicate is a mapping", n.Line)
	}
	forms := map[string]*yaml.Node{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		forms[n.Content[i].Value] = n.Content[i+1]
	}
	ops := []string{"eq", "ne", "in", "regex", "exists"}
	var form string
	for k := range forms {
		switch k {
		case "attr", "all", "any", "not", "related", "eq", "ne", "in", "regex", "exists":
		default:
			return fmt.Errorf("line %d: %q is not part of the predicate language", n.Line, k)
		}
	}
	count := 0
	for _, f := range []string{"attr", "all", "any", "not", "related"} {
		if _, ok := forms[f]; ok {
			count++
			form = f
		}
	}
	if count != 1 {
		return fmt.Errorf("line %d: a predicate has exactly one of attr, all, any, not, related", n.Line)
	}
	switch form {
	case "attr":
		p.Attr = forms["attr"].Value
		var used []string
		for _, op := range ops {
			if v, ok := forms[op]; ok {
				used = append(used, op)
				p.Op = op
				switch op {
				case "in":
					var vs []any
					if err := v.Decode(&vs); err != nil {
						return fmt.Errorf("line %d: in takes a list", v.Line)
					}
					p.Values = vs
				case "exists":
					var b bool
					if err := v.Decode(&b); err != nil {
						return fmt.Errorf("line %d: exists takes true or false", v.Line)
					}
					p.Value = b
				case "regex":
					re, err := regexp.Compile(v.Value)
					if err != nil {
						return fmt.Errorf("line %d: %w", v.Line, err)
					}
					p.re = re
					p.Value = v.Value
				default:
					var x any
					if err := v.Decode(&x); err != nil {
						return err
					}
					p.Value = x
				}
			}
		}
		if len(used) != 1 {
			return fmt.Errorf("line %d: attr %q needs exactly one of eq, ne, in, regex, exists", n.Line, p.Attr)
		}
	case "all", "any":
		var list []*Pred
		if err := forms[form].Decode(&list); err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("line %d: %s needs at least one predicate", n.Line, form)
		}
		if form == "all" {
			p.All = list
		} else {
			p.Any = list
		}
	case "not":
		var inner Pred
		if err := forms["not"].Decode(&inner); err != nil {
			return err
		}
		p.Not = &inner
	case "related":
		var raw struct {
			Rel     string `yaml:"rel"`
			Where   *Pred  `yaml:"where"`
			CountGE *int   `yaml:"count_ge"`
			CountLE *int   `yaml:"count_le"`
		}
		if err := strictDecodeNode(forms["related"], &raw); err != nil {
			return err
		}
		if raw.Rel == "" {
			return fmt.Errorf("line %d: related needs rel", n.Line)
		}
		p.Related = &Related{Rel: raw.Rel, Where: raw.Where, CountGE: raw.CountGE, CountLE: raw.CountLE}
	}
	return nil
}

func strictDecodeNode(n *yaml.Node, v any) error {
	b, err := yaml.Marshal(n)
	if err != nil {
		return err
	}
	return strictDecode(b, v)
}

// check walks a predicate against the schema from the given kind and reports every attribute or relationship it names that does
// not exist, so a rule cannot silently test something the model does not have.
func (p *Pred) check(s *Schema, kind string) error {
	k, ok := s.Kinds[kind]
	if !ok {
		return fmt.Errorf("kind %s is not in the schema", kind)
	}
	switch {
	case p.Attr != "":
		if k.Attr(p.Attr) == nil {
			return fmt.Errorf("%s has no attribute %q", kind, p.Attr)
		}
	case p.All != nil:
		for _, q := range p.All {
			if err := q.check(s, kind); err != nil {
				return err
			}
		}
	case p.Any != nil:
		for _, q := range p.Any {
			if err := q.check(s, kind); err != nil {
				return err
			}
		}
	case p.Not != nil:
		return p.Not.check(s, kind)
	case p.Related != nil:
		r := k.Rel(p.Related.Rel)
		if r == nil {
			return fmt.Errorf("%s has no relationship %q", kind, p.Related.Rel)
		}
		if p.Related.Where != nil {
			return p.Related.Where.check(s, r.Peer)
		}
	}
	return nil
}

// eval reports whether the predicate holds for an object.
func (p *Pred) eval(s *Schema, g *Graph, o *Object) bool {
	k := s.Kinds[o.Kind]
	switch {
	case p.Attr != "":
		v := value(k, o, p.Attr)
		switch p.Op {
		case "exists":
			want, _ := p.Value.(bool)
			return (v != nil && str(v) != "") == want
		case "eq":
			return v != nil && str(v) == str(p.Value)
		case "ne":
			return v == nil || str(v) != str(p.Value)
		case "in":
			for _, x := range p.Values {
				if v != nil && str(v) == str(x) {
					return true
				}
			}
			return false
		case "regex":
			return v != nil && p.re.MatchString(str(v))
		}
		return false
	case p.All != nil:
		for _, q := range p.All {
			if !q.eval(s, g, o) {
				return false
			}
		}
		return true
	case p.Any != nil:
		for _, q := range p.Any {
			if q.eval(s, g, o) {
				return true
			}
		}
		return false
	case p.Not != nil:
		return !p.Not.eval(s, g, o)
	case p.Related != nil:
		n := len(related(s, g, o, p.Related.Rel, p.Related.Where))
		ge := 1
		if p.Related.CountGE != nil {
			ge = *p.Related.CountGE
		} else if p.Related.CountLE != nil {
			ge = 0
		}
		if n < ge {
			return false
		}
		return p.Related.CountLE == nil || n <= *p.Related.CountLE
	}
	return false
}

// related returns the resolved peers of o over rel that satisfy where (nil where matches every peer).
func related(s *Schema, g *Graph, o *Object, rel string, where *Pred) []*Object {
	var out []*Object
	for _, ref := range o.Rels[rel] {
		peer := g.Get(ref.Key)
		if peer == nil {
			continue
		}
		if where == nil || where.eval(s, g, peer) {
			out = append(out, peer)
		}
	}
	return out
}
