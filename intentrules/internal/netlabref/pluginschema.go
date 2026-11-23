package netlabref

// pluginschema.go -- tell the author, from netlab itself, what a plugin accepts.
//
// WHY. A plugin's block (`fabric:`, `files:`, ...) is validated by netlab against
// the `attributes:` section of the plugin's own defaults.yml, and the plugin's
// Python then trusts those types. An author who guesses the shape of a plugin
// block (dicts where the plugin wants integers) gets either a readable netlab
// error or, when the plugin is reached before validation, a Python traceback. The
// model used to be handed neither the types nor a readable crash, and the
// cheapest "fix" that compiles is to delete what the plugin could not expand.
//
// Everything here is DERIVED from the generated capability manifest
// (netlab_capabilities.json, itself generated from the patched netlab tree). No
// plugin is named in this file, and there is no per-plugin advice: the types and
// the `required` flags are netlab's own.

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcapsgen"
	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabdocs"
)

// PluginSchema is one enabled plugin's declared attribute surface.
type PluginSchema struct {
	Plugin string `json:"plugin"`
	// Known is false when the manifest has no plugin of that name (a typo, or a
	// plugin from a tree we do not ship). Attributes is then meaningless.
	Known      bool              `json:"known"`
	Attributes []PluginAttribute `json:"attributes,omitempty"`
	// Doc is the netlab_docs path that explains the plugin, when the corpus has one.
	Doc  string `json:"doc,omitempty"`
	Note string `json:"note,omitempty"`
}

// PluginAttribute is netlab's declaration of one attribute of a plugin.
type PluginAttribute struct {
	Scope       string   `json:"scope"`
	Path        string   `json:"path"`
	Type        string   `json:"type,omitempty"`
	Required    bool     `json:"required,omitempty"`
	ValidValues []string `json:"validValues,omitempty"`
}

// EnabledPlugins returns the plugin names a topology enables with its top-level
// `plugin:` key (a list, a single string, or a mapping whose keys are the names),
// in written order and de-duplicated. Unparseable YAML enables nothing: the
// caller is describing a topology netlab will judge on its own.
func EnabledPlugins(topologyYAML string) []string {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(topologyYAML), &doc); err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	add := func(v any) {
		name := strings.TrimSpace(fmt.Sprint(v))
		if name == "" || name == "<nil>" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	switch p := doc["plugin"].(type) {
	case string:
		add(p)
	case []any:
		for _, v := range p {
			add(v)
		}
	case map[string]any:
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			add(k)
		}
	}
	return out
}

// SchemaFor builds the schema for one plugin from the manifest.
func SchemaFor(plugin string) PluginSchema {
	s := PluginSchema{Plugin: plugin}
	attrs, ok := netlabcaps.PluginAttributes(plugin)
	if !ok {
		s.Note = fmt.Sprintf("netlab has no plugin named %q in this tree; check the spelling with netlab_capabilities.", plugin)
		return s
	}
	s.Known = true
	for _, a := range attrs {
		s.Attributes = append(s.Attributes, PluginAttribute{
			Scope: a.Scope, Path: a.Path, Type: a.Type, Required: a.Required, ValidValues: a.ValidValues,
		})
	}
	sort.SliceStable(s.Attributes, func(i, j int) bool {
		if s.Attributes[i].Scope != s.Attributes[j].Scope {
			return s.Attributes[i].Scope < s.Attributes[j].Scope
		}
		return s.Attributes[i].Path < s.Attributes[j].Path
	})
	if len(s.Attributes) == 0 {
		s.Note = "netlab declares no attributes for this plugin; its configuration is read from the keys it documents."
	}
	if doc := "plugins/" + plugin + ".md"; docExists(doc) {
		s.Doc = doc
	}
	return s
}

func docExists(path string) bool {
	_, ok := netlabdocs.Read(path)
	return ok
}

// SchemasForTopology returns one schema per plugin the topology enables, or nil
// when it enables none (the compile result then carries no `pluginAttributes`).
func SchemasForTopology(topologyYAML string) []PluginSchema {
	names := EnabledPlugins(topologyYAML)
	if len(names) == 0 || !netlabcaps.Loaded() {
		return nil
	}
	out := make([]PluginSchema, 0, len(names))
	for _, n := range names {
		out = append(out, SchemaFor(n))
	}
	return out
}

// Describe renders attributes as the one-line form used in diagnostics and in
// netlab_capabilities: `scope: path (type, required) one of [..], ...`.
func Describe(attrs []PluginAttribute) string {
	byScope := map[string][]string{}
	for _, a := range attrs {
		d := a.Path
		var tags []string
		if a.Type != "" {
			tags = append(tags, a.Type)
		}
		if a.Required {
			tags = append(tags, "required")
		}
		if len(tags) > 0 {
			d += " (" + strings.Join(tags, ", ") + ")"
		}
		if len(a.ValidValues) > 0 {
			d += " one of [" + strings.Join(a.ValidValues, ", ") + "]"
		}
		byScope[a.Scope] = append(byScope[a.Scope], d)
	}
	scopes := make([]string, 0, len(byScope))
	for k := range byScope {
		scopes = append(scopes, k)
	}
	sort.Strings(scopes)
	parts := make([]string, 0, len(scopes))
	for _, k := range scopes {
		sort.Strings(byScope[k])
		parts = append(parts, k+": "+strings.Join(byScope[k], ", "))
	}
	return strings.Join(parts, "; ")
}

func attrsFromManifest(in []netlabcapsgen.Attribute) []PluginAttribute {
	out := make([]PluginAttribute, 0, len(in))
	for _, a := range in {
		out = append(out, PluginAttribute{Scope: a.Scope, Path: a.Path, Type: a.Type, Required: a.Required, ValidValues: a.ValidValues})
	}
	return out
}
