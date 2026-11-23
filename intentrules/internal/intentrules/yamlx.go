package intentrules

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Helpers for reading a netlab document the way netlab does: aliases followed, and a dotted key (vlan.access: red) read the same
// as the nested form (vlan: {access: red}), which netlab accepts everywhere.

func deref(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func isMap(n *yaml.Node) bool { n = deref(n); return n != nil && n.Kind == yaml.MappingNode }
func isSeq(n *yaml.Node) bool { n = deref(n); return n != nil && n.Kind == yaml.SequenceNode }

func isScalar(n *yaml.Node) bool { n = deref(n); return n != nil && n.Kind == yaml.ScalarNode }

func isNull(n *yaml.Node) bool {
	n = deref(n)
	return n == nil || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

// mapPairs returns key/value pairs in document order, expanding YAML merge keys (<<) the way a loader does.
func mapPairs(n *yaml.Node) [][2]*yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	var out [][2]*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Value == "<<" && k.Tag == "!!merge" {
			if isMap(v) {
				out = append(out, mapPairs(v)...)
			} else if isSeq(v) {
				for _, m := range deref(v).Content {
					out = append(out, mapPairs(m)...)
				}
			}
			continue
		}
		out = append(out, [2]*yaml.Node{k, v})
	}
	return out
}

// get reads one key.
func get(n *yaml.Node, key string) *yaml.Node {
	for _, p := range mapPairs(n) {
		if p[0].Value == key {
			return deref(p[1])
		}
	}
	return nil
}

// getPath reads a dotted path through nested mappings, also accepting a flat dotted key at any level.
func getPath(n *yaml.Node, path string) *yaml.Node {
	segs := strings.Split(path, ".")
	return getSegs(n, segs)
}

func getSegs(n *yaml.Node, segs []string) *yaml.Node {
	if !isMap(n) {
		return nil
	}
	for i := 1; i <= len(segs); i++ {
		key := strings.Join(segs[:i], ".")
		if v := get(n, key); v != nil {
			if i == len(segs) {
				return v
			}
			if r := getSegs(v, segs[i:]); r != nil {
				return r
			}
		}
	}
	return nil
}

// scalar returns the text of a scalar node, "" otherwise.
func scalar(n *yaml.Node) string {
	n = deref(n)
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

// isFalse reports an explicit false.
func isFalse(n *yaml.Node) bool {
	n = deref(n)
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!bool" && strings.EqualFold(n.Value, "false")
}

func isTrue(n *yaml.Node) bool {
	n = deref(n)
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!bool" && strings.EqualFold(n.Value, "true")
}

// stringList reads a sequence of scalars, or a single scalar as a one-element list.
func stringList(n *yaml.Node) []string {
	n = deref(n)
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		if s := scalar(n); s != "" {
			return []string{s}
		}
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(n.Content))
	for _, c := range n.Content {
		if s := scalar(c); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// keysOf returns the mapping keys in order.
func keysOf(n *yaml.Node) []string {
	var out []string
	for _, p := range mapPairs(n) {
		out = append(out, p[0].Value)
	}
	return out
}
