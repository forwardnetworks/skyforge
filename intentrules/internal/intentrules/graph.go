package intentrules

import "fmt"

// Object is one instance of a schema kind, built by the loader from a document.
type Object struct {
	Kind string
	Key  string
	// Path is where the object came from in the authored document ("nodes.r1", "links[2]"), so a finding can point at it.
	Path  string
	Attrs map[string]any // attribute -> string, bool, int or nil (unset)
	Rels  map[string][]Ref
}

// Ref is one end of a relationship. Key is empty when the reference could not be resolved to an object; Name is what the
// document said, for the message.
type Ref struct {
	Key  string
	Name string
}

// Graph is every object the loader built, plus what it could not judge.
type Graph struct {
	Objects []*Object
	byKey   map[string]*Object
	// Skipped lists, per rule or constraint id, why the document could not be judged by it. A clean result with a skip is
	// "did not look", never "looked and found nothing".
	Skipped map[string]string
	seq     int
}

// NewGraph returns an empty graph.
func NewGraph() *Graph { return &Graph{byKey: map[string]*Object{}, Skipped: map[string]string{}} }

// Add inserts an object; the key must be unique across the graph.
func (g *Graph) Add(o *Object) *Object {
	if o.Attrs == nil {
		o.Attrs = map[string]any{}
	}
	if o.Rels == nil {
		o.Rels = map[string][]Ref{}
	}
	if o.Key == "" {
		g.seq++
		o.Key = fmt.Sprintf("%s#%d", o.Kind, g.seq)
	}
	if _, dup := g.byKey[o.Key]; dup {
		g.seq++
		o.Key = fmt.Sprintf("%s#%d", o.Key, g.seq)
	}
	g.byKey[o.Key] = o
	g.Objects = append(g.Objects, o)
	return o
}

// Get returns the object with this key.
func (g *Graph) Get(key string) *Object {
	if key == "" {
		return nil
	}
	return g.byKey[key]
}

// Skip records that something could not be judged.
func (g *Graph) Skip(id, reason string) {
	if _, ok := g.Skipped[id]; !ok {
		g.Skipped[id] = reason
	}
}

// Link adds a resolved reference.
func (o *Object) Link(rel string, target *Object) {
	o.Rels[rel] = append(o.Rels[rel], Ref{Key: target.Key, Name: str(target.Attrs["name"])})
}

// Dangling adds a reference that resolves to nothing.
func (o *Object) Dangling(rel, name string) { o.Rels[rel] = append(o.Rels[rel], Ref{Name: name}) }

func str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}
