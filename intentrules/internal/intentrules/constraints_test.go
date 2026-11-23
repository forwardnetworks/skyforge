package intentrules

import (
	"strings"
	"testing"
)

// A schema of our own for the evaluator tests, in Infrahub's language, small enough to read. Each constraint below is tested
// from what Infrahub's schema says it means, with a graph that breaks it and one that does not.
const testSchema = `
version: "1.0"
generics:
  - name: Thing
    namespace: Test
    uniqueness_constraints:
      - ["owner", "tag__value"]
    attributes:
      - name: tag
        kind: Text
        optional: true
    relationships:
      - name: owner
        peer: TestOwner
        kind: Attribute
        cardinality: one
        optional: false
        identifier: testowner__testthing
nodes:
  - name: Owner
    namespace: Test
    attributes:
      - name: name
        kind: Text
        regex: '^[a-z]+$'
      - name: level
        kind: Number
        optional: true
        parameters: { min_value: 1, max_value: 5 }
      - name: tier
        kind: Dropdown
        optional: true
        choices:
          - { name: gold, label: Gold }
          - { name: tin, label: Tin }
  - name: Pair
    namespace: Test
    inherit_from: [TestThing]
    relationships:
      - name: ends
        peer: TestOwner
        kind: Attribute
        cardinality: many
        min_count: 2
        max_count: 2
        optional: false
        identifier: testpair__testowner
  - name: Solo
    namespace: Test
    inherit_from: [TestThing]
`

func testEngineSchema(t *testing.T) *Schema {
	t.Helper()
	s, err := LoadSchema("test", []byte(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func owner(g *Graph, name string, attrs map[string]any) *Object {
	if attrs == nil {
		attrs = map[string]any{}
	}
	attrs["name"] = name
	return g.Add(&Object{Kind: "TestOwner", Key: "o:" + name, Attrs: attrs})
}

func constraintSet(vs []Violation) string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Constraint+":"+v.Member)
	}
	return strings.Join(out, ",")
}

func TestConstraintsEachOneFiresAndTheCleanGraphIsClean(t *testing.T) {
	s := testSchema_(t)
	cases := []struct {
		name  string
		build func(g *Graph)
		want  string
	}{
		{"clean", func(g *Graph) {
			a, b := owner(g, "alpha", map[string]any{"level": 3, "tier": "gold"}), owner(g, "beta", nil)
			p := g.Add(&Object{Kind: "TestPair", Attrs: map[string]any{"tag": "x"}})
			p.Link("owner", a)
			p.Link("ends", a)
			p.Link("ends", b)
		}, ""},
		{"regex", func(g *Graph) { owner(g, "Bad_1", nil) }, "regex:name"},
		{"enum", func(g *Graph) { owner(g, "ok", map[string]any{"tier": "bronze"}) }, "enum:tier"},
		{"range high", func(g *Graph) { owner(g, "ok", map[string]any{"level": 9}) }, "range:level"},
		{"range low", func(g *Graph) { owner(g, "ok", map[string]any{"level": 0}) }, "range:level"},
		{"required relationship", func(g *Graph) { g.Add(&Object{Kind: "TestSolo"}) }, "required:owner"},
		{"dangling peer", func(g *Graph) {
			s := g.Add(&Object{Kind: "TestSolo"})
			s.Dangling("owner", "ghost")
		}, "ref:owner"},
		{"wrong peer kind", func(g *Graph) {
			s := g.Add(&Object{Kind: "TestSolo"})
			s2 := g.Add(&Object{Kind: "TestSolo", Key: "other"})
			s2.Link("owner", owner(g, "alpha", nil))
			s.Link("owner", s2) // a Solo where an Owner is required
		}, "ref:owner"},
		{"cardinality one", func(g *Graph) {
			s := g.Add(&Object{Kind: "TestSolo"})
			s.Link("owner", owner(g, "alpha", nil))
			s.Link("owner", owner(g, "beta", nil))
		}, "cardinality:owner"},
		// Infrahub 1.11.11 stores these; this layer is where they are caught.
		{"min_count zero ends", func(g *Graph) {
			p := g.Add(&Object{Kind: "TestPair"})
			p.Link("owner", owner(g, "alpha", nil))
		}, "min_count:ends"},
		{"min_count one end", func(g *Graph) {
			a := owner(g, "alpha", nil)
			p := g.Add(&Object{Kind: "TestPair"})
			p.Link("owner", a)
			p.Link("ends", a)
		}, "min_count:ends"},
		{"max_count three ends", func(g *Graph) {
			a := owner(g, "alpha", nil)
			p := g.Add(&Object{Kind: "TestPair"})
			p.Link("owner", a)
			p.Link("ends", a)
			p.Link("ends", a)
			p.Link("ends", a)
		}, "max_count:ends"},
		// declared on the generic: holds across the nodes that inherit it, not within each node
		{"unique across kinds", func(g *Graph) {
			a := owner(g, "alpha", nil)
			x := g.Add(&Object{Kind: "TestSolo", Attrs: map[string]any{"tag": "dup"}})
			y := g.Add(&Object{Kind: "TestPair", Attrs: map[string]any{"tag": "dup"}})
			x.Link("owner", a)
			y.Link("owner", a)
			y.Link("ends", a)
			y.Link("ends", owner(g, "beta", nil))
		}, "unique:tag"},
		{"unique but different owner", func(g *Graph) {
			x := g.Add(&Object{Kind: "TestSolo", Attrs: map[string]any{"tag": "dup"}})
			y := g.Add(&Object{Kind: "TestSolo", Attrs: map[string]any{"tag": "dup"}})
			x.Link("owner", owner(g, "alpha", nil))
			y.Link("owner", owner(g, "beta", nil))
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewGraph()
			c.build(g)
			if got := constraintSet(s.Constraints(g)); got != c.want {
				t.Errorf("violations = %q, want %q", got, c.want)
			}
		})
	}
}

func testSchema_(t *testing.T) *Schema { return testEngineSchema(t) }

func TestSchemaLoaderRefusesWhatInfrahubRefuses(t *testing.T) {
	cases := map[string]string{
		"unknown peer":             strings.Replace(testSchema, "peer: TestOwner\n        kind: Attribute\n        cardinality: one", "peer: TestNobody\n        kind: Attribute\n        cardinality: one", 1),
		"inherit from a node":      strings.Replace(testSchema, "inherit_from: [TestThing]\n    relationships:\n      - name: ends", "inherit_from: [TestOwner]\n    relationships:\n      - name: ends", 1),
		"uniqueness names a ghost": strings.Replace(testSchema, `["owner", "tag__value"]`, `["owner", "nope__value"]`, 1),
		"no version":               strings.Replace(testSchema, `version: "1.0"`, "", 1),
	}
	for name, doc := range cases {
		if _, err := LoadSchema(name, []byte(doc)); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

func TestPredicateLanguageEvaluates(t *testing.T) {
	s := testSchema_(t)
	g := NewGraph()
	a := owner(g, "alpha", map[string]any{"level": 3, "tier": "gold"})
	b := owner(g, "beta", nil)
	p := g.Add(&Object{Kind: "TestPair"})
	p.Link("owner", a)
	p.Link("ends", a)
	p.Link("ends", b)
	parse := func(doc string) *Pred {
		var q Pred
		if err := strictDecode([]byte(doc), &q); err != nil {
			// Pred decodes through UnmarshalYAML; strictDecode goes through the same decoder
			t.Fatalf("%s: %v", doc, err)
		}
		return &q
	}
	truths := map[string]bool{
		`{attr: tier, eq: gold}`:                                          true,
		`{attr: tier, ne: gold}`:                                          false,
		`{attr: tier, in: [gold, tin]}`:                                   true,
		`{attr: name, regex: "^al"}`:                                      true,
		`{attr: tier, exists: true}`:                                      true,
		`{all: [{attr: tier, eq: gold}, {attr: name, eq: alpha}]}`:        true,
		`{any: [{attr: tier, eq: tin}, {attr: name, eq: alpha}]}`:         true,
		`{not: {attr: tier, eq: gold}}`:                                   false,
		`{all: [{attr: tier, eq: gold}, {not: {attr: name, eq: alpha}}]}`: false,
	}
	for doc, want := range truths {
		if got := parse(doc).eval(s, g, a); got != want {
			t.Errorf("%s on alpha = %v, want %v", doc, got, want)
		}
	}
	rel := map[string]bool{
		`{related: {rel: ends}}`:                                             true,
		`{related: {rel: ends, count_ge: 2}}`:                                true,
		`{related: {rel: ends, count_ge: 3}}`:                                false,
		`{related: {rel: ends, count_le: 1}}`:                                false,
		`{related: {rel: ends, where: {attr: tier, eq: gold}, count_ge: 1}}`: true,
		`{related: {rel: ends, where: {attr: tier, eq: gold}, count_ge: 2}}`: false,
		`{related: {rel: owner, where: {attr: name, eq: beta}}}`:             false,
	}
	for doc, want := range rel {
		if got := parse(doc).eval(s, g, p); got != want {
			t.Errorf("%s on the pair = %v, want %v", doc, got, want)
		}
	}
}
