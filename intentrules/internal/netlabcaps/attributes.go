package netlabcaps

import (
	"sort"
	"strings"
)

// FactNetlabAttributes is the fact key for the AUTHORABLE ATTRIBUTE
// surface — which attribute each module accepts, at which scope, and where a
// module name may itself be used as a boolean.
//
// It is a third axis, separate from the two keys this corpus already serves:
//
//	netlab.devices / .modules / .plugins  what does THIS DEVICE render?
//	netlab.capabilities                   what NAMES exist at all?
//	netlab.attributes                     what may be WRITTEN, and where?
//
// The third is the one a correct author fails on. capabilityCatalogue() is
// names-only by design, so nothing rendered anywhere answered "is this legal on
// a link" — and a consumer that cannot answer it either guesses or calls valid
// authoring a modelling gap.
//
// EVERY LINE IS DERIVED. There is no worked example, no anecdote and no advice
// in this rendering: it is a projection of netlab_capabilities.json, which is
// itself generated from the PATCHED tree. Prose about what
// people typically get wrong belongs in a skill body, where a human edits it;
// putting it here would make a generated artifact un-regenerable without losing
// it.
const FactNetlabAttributes = "netlab.attributes"

// AttributeFact answers FactNetlabAttributes, in the shape a fact function
// expects. Like Fact, it returns false rather than an empty string when the
// manifest did not load: "unreadable" and "no attributes exist" are different
// states and must not render the same.
func AttributeFact(key string) (string, bool) {
	if key != FactNetlabAttributes {
		return "", false
	}
	load()
	if !Loaded() {
		return "", false
	}
	return attributeCatalogue(), true
}

// authorableScopeOrder is the order scopes are rendered in. It is fixed rather
// than sorted so the rendering is stable across regenerations, and it leads with
// the scopes an author writes most.
var authorableScopeOrder = []string{"link", "interface", "node", "global", "loopback", "vlan", "vrf", "pool"}

// attributeCatalogue renders the attribute surface, grouped by scope.
//
// Grouped BY SCOPE and not by module on purpose: the question being answered is
// "what may I write HERE", which is a property of the position in the topology,
// not of the module. A module-major rendering makes the reader scan all 22
// modules to answer it.
func attributeCatalogue() string {
	m := Manifest()

	var b strings.Builder
	b.WriteString("Generated from the PATCHED netlab tree; upstream netlab differs. These are the attribute names netlab ")
	b.WriteString("VALIDATES against — anything else fails the real compile.\n")

	if len(m.CanBeFalse) > 0 {
		scopes := append([]string(nil), m.CanBeFalse...)
		sort.Strings(scopes)
		b.WriteString("\n**Any module name may be set to `false` at these scopes**, which disables that module there: ")
		b.WriteString("`" + strings.Join(scopes, "`, `") + "`. So `<module>: false` (for example on a single link) is ")
		b.WriteString("VALID AUTHORING for every module listed below, whether or not that module declares link attributes ")
		b.WriteString("of its own.\n")
	}

	byScope := map[string]map[string][]string{}
	enums := map[string][]string{}
	for _, mod := range m.Modules {
		for _, a := range mod.Attributes {
			sc, path := strings.TrimSpace(a.Scope), strings.TrimSpace(a.Path)
			if sc == "" || path == "" {
				continue
			}
			if byScope[sc] == nil {
				byScope[sc] = map[string][]string{}
			}
			byScope[sc][mod.Name] = append(byScope[sc][mod.Name], path)
			if len(a.ValidValues) > 0 {
				enums[mod.Name+"."+path] = a.ValidValues
			}
		}
	}

	seen := map[string]bool{}
	render := func(scope string) {
		mods := byScope[scope]
		if len(mods) == 0 {
			return
		}
		seen[scope] = true
		names := make([]string, 0, len(mods))
		for n := range mods {
			names = append(names, n)
		}
		sort.Strings(names)
		b.WriteString("\n**" + scope + "** attributes, written as `<module>.<path>`:\n")
		for _, n := range names {
			paths := mods[n]
			sort.Strings(paths)
			b.WriteString("- " + n + ": " + strings.Join(dedupe(paths), ", ") + "\n")
		}
	}
	for _, sc := range authorableScopeOrder {
		render(sc)
	}
	// Any scope the manifest carries that the fixed order does not name is still
	// rendered. A scope silently dropped because a generator learned a new one is
	// exactly the drift this file exists to prevent.
	rest := make([]string, 0, len(byScope))
	for sc := range byScope {
		if !seen[sc] {
			rest = append(rest, sc)
		}
	}
	sort.Strings(rest)
	for _, sc := range rest {
		render(sc)
	}

	var trues []string
	for _, mod := range m.Modules {
		if len(mod.CanBeTrue) > 0 {
			s := append([]string(nil), mod.CanBeTrue...)
			sort.Strings(s)
			trues = append(trues, mod.Name+": "+strings.Join(s, ", "))
		}
	}
	if len(trues) > 0 {
		sort.Strings(trues)
		b.WriteString("\n**`<module>: true` is additionally legal at**: " + strings.Join(trues, "; ") + "\n")
	}

	// The top-level blocks are rendered as their own section, not folded into
	// the "global" scope above: they are not `<module>.<path>` -- an author
	// writes `otg:` at the top of the topology, not `module: [otg]` -- and a
	// reader that saw one under "global attributes, written as <module>.<path>"
	// would do exactly the wrong thing with it.
	if len(m.SkyforgeGlobals) > 0 {
		b.WriteString("\n**Skyforge top-level blocks** (written at the top of the topology as `<block>:`; NOT modules or plugins; " +
			"`[]` marks one entry of a list; validated by netlab, consumed by Skyforge):\n")
		for _, g := range m.SkyforgeGlobals {
			paths := make([]string, 0, len(g.Attributes))
			for _, a := range g.Attributes {
				if p := strings.TrimSpace(a.Path); p != "" {
					paths = append(paths, p)
					if len(a.ValidValues) > 0 {
						enums[g.Name+"."+p] = a.ValidValues
					}
				}
			}
			sort.Strings(paths)
			b.WriteString("- " + g.Name + ": " + strings.Join(dedupe(paths), ", ") + "\n")
		}
	}

	if len(enums) > 0 {
		keys := make([]string, 0, len(enums))
		for k := range enums {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("\n**Declared enums** (the only accepted values):\n")
		for _, k := range keys {
			b.WriteString("- " + k + ": " + strings.Join(enums[k], " | ") + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// dedupe collapses repeated paths. A module may declare the same path in one
// scope more than once (an alias, or a copy: marker), and rendering it twice
// reads as two different attributes.
func dedupe(in []string) []string {
	out := in[:0:0]
	var last string
	for i, s := range in {
		if i > 0 && s == last {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}
