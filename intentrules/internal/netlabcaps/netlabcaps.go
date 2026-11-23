// Package netlabcaps publishes the generated netlab CAPABILITY MANIFEST: what
// the patched netlab can be asked to do — its modules, their attributes and
// valid values, its plugins, and which devices declare and render them.
//
// The file is generated from a PATCHED netlab tree and
// embedded here (go:embed cannot reach across package directories, which is why
// the artifact lives beside its reader — the same arrangement as
// internal/taskengine/netlab_device_defaults.json).
//
// WHY IT IS GENERATED. We run netlab with a patch series on top of a pinned
// upstream. Anything a human writes down about that surface drifts, and the
// evidence is in this repo: an example topology once declared
// `plugin: [tunnel.dmvpn, ospf.instances, bgp.rr_group]` and none of the three
// exists — the real names are tunnel.gre/tunnel.wireguard, ospf.areas, and
// bgp.domain/bgp.policy/bgp.session. That topology cannot compile, and nothing
// noticed, because until now the server had no machine-readable statement of
// which plugins exist.
//
// STALENESS IS THE RISK THIS PACKAGE CARRIES. The manifest is a photograph of
// the patched tree at generation time; when the patch series moves, it is wrong
// and nothing here can tell. See TestManifestIsNotStale in netlabcapsgen for
// what does check, and the handoff note for the superproject gate that should.
package netlabcaps

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcapsgen"
)

// webUIOverridesJSON is the committed per-device web UI override list (see the
// file's _comment). It narrows what the kne service map implies; it never widens it.
//
//go:embed web_ui_overrides.json
var webUIOverridesJSON []byte

type webUIOverride struct {
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
	// Scheme and Port say where this device's web UI listens when it is not the
	// default https/443. Both are optional; a missing Port defaults from Scheme.
	Scheme string `json:"scheme,omitempty"`
	Port   int    `json:"port,omitempty"`
	// PathPrefixRewrite marks a device whose web UI assumes it is served at the
	// origin root (root-absolute links, <base href="/">, Path=/ cookies) and so
	// breaks under the proxy's path prefix. Present means "the proxy rewrites its
	// responses and injects a client shim"; absent means pass-through.
	PathPrefixRewrite *PathPrefixRewrite `json:"pathPrefixRewrite,omitempty"`
}

// PathPrefixRewrite is the device data the browser proxy's prefix rewrite needs
// beyond the generic rules (base href, href/src/action attributes, Set-Cookie
// Path, Location). Each entry is a root-absolute path that the device's own
// markup or script uses as a whole quoted string, typically as a navigation
// target (window.location = "/logout") that the client shim cannot intercept.
type PathPrefixRewrite struct {
	// HTMLLiterals are rewritten in text/html bodies: inline scripts and the
	// document.location="..." bodies a login endpoint returns.
	HTMLLiterals []string `json:"htmlLiterals,omitempty"`
	// ScriptLiterals are rewritten in JavaScript bodies. Keep this list to exact
	// whole-string paths: router paths that the framework resolves against the
	// base href must NOT be listed (they would be prefixed twice).
	ScriptLiterals []string `json:"scriptLiterals,omitempty"`
	// StripPrefixQueryParams names query parameters whose value is a path the
	// device's own page computed from window.location (a post-login "redir"). The
	// browser sees the prefixed path; the device's router wants it without. The
	// proxy removes the prefix from those values on the way in.
	StripPrefixQueryParams []string `json:"stripPrefixQueryParams,omitempty"`
	// ScriptReplacements are exact-substring replacements applied to JavaScript
	// bodies, for the one thing a literal rewrite cannot reach: a framework that
	// fixes its own base path in code. A replacement whose Old text is absent is
	// a no-op (a different device build then simply behaves as before). New may
	// read window.__skyforgeBasePath, which the injected shim sets to the prefix.
	ScriptReplacements []ScriptReplacement `json:"scriptReplacements,omitempty"`
}

// ScriptReplacement is one exact-substring replacement (see ScriptReplacements).
type ScriptReplacement struct {
	Old string `json:"old"`
	New string `json:"new"`
}

// Default web UI endpoint for a device with no override.
const (
	DefaultWebUIScheme = "https"
	DefaultWebUIPort   = 443
)

var (
	webUIOnce      sync.Once
	webUIOverrides map[string]webUIOverride
)

func loadWebUIOverrides() map[string]webUIOverride {
	webUIOnce.Do(func() {
		var doc struct {
			Devices map[string]webUIOverride `json:"devices"`
		}
		_ = json.Unmarshal(webUIOverridesJSON, &doc)
		webUIOverrides = map[string]webUIOverride{}
		for k, v := range doc.Devices {
			webUIOverrides[strings.ToLower(strings.TrimSpace(k))] = v
		}
	})
	return webUIOverrides
}

// WebUIEndpoint returns the scheme and port the browser proxy should dial for a
// device's web UI when the caller names neither: the per-device override in
// web_ui_overrides.json, else https/443. An override with only a scheme gets that
// scheme's default port (http 80, https 443); an unrecognised scheme or an
// out-of-range port is ignored rather than trusted.
func WebUIEndpoint(device string) (scheme string, port int) {
	scheme, port = DefaultWebUIScheme, DefaultWebUIPort
	o, ok := loadWebUIOverrides()[strings.ToLower(strings.TrimSpace(device))]
	if !ok {
		return scheme, port
	}
	switch strings.ToLower(strings.TrimSpace(o.Scheme)) {
	case "http":
		scheme, port = "http", 80
	case "https":
		scheme, port = "https", 443
	}
	if o.Port > 0 && o.Port <= 65535 {
		port = o.Port
	}
	return scheme, port
}

// WebUIPathPrefixRewrite returns the prefix-rewrite data for a device, or nil
// when its web UI works under a path prefix as is.
func WebUIPathPrefixRewrite(device string) *PathPrefixRewrite {
	o, ok := loadWebUIOverrides()[strings.ToLower(strings.TrimSpace(device))]
	if !ok || !o.Supported || o.PathPrefixRewrite == nil {
		return nil
	}
	src := o.PathPrefixRewrite
	return &PathPrefixRewrite{
		HTMLLiterals:           append([]string(nil), src.HTMLLiterals...),
		ScriptLiterals:         append([]string(nil), src.ScriptLiterals...),
		StripPrefixQueryParams: append([]string(nil), src.StripPrefixQueryParams...),
		ScriptReplacements:     append([]ScriptReplacement(nil), src.ScriptReplacements...),
	}
}

// WebUIUnsupportedReason returns the user-facing reason a device has no usable
// web UI despite its service map, or "" when no override marks it unsupported.
func WebUIUnsupportedReason(device string) string {
	o, ok := loadWebUIOverrides()[strings.ToLower(strings.TrimSpace(device))]
	if !ok || o.Supported {
		return ""
	}
	if strings.TrimSpace(o.Reason) == "" {
		return "The web UI is not available for this device on this image."
	}
	return strings.TrimSpace(o.Reason)
}

var (
	once     sync.Once
	manifest netlabcapsgen.Manifest
	index    struct {
		modules map[string]*netlabcapsgen.Module
		plugins map[string]*netlabcapsgen.Plugin
		devices map[string]*netlabcapsgen.Device
		globals map[string]*netlabcapsgen.SkyforgeGlobal
	}
)

func load() {
	once.Do(func() {
		_ = json.Unmarshal(manifestJSON, &manifest)
		index.modules = map[string]*netlabcapsgen.Module{}
		index.plugins = map[string]*netlabcapsgen.Plugin{}
		index.devices = map[string]*netlabcapsgen.Device{}
		for i := range manifest.Modules {
			index.modules[manifest.Modules[i].Name] = &manifest.Modules[i]
		}
		for i := range manifest.Plugins {
			index.plugins[manifest.Plugins[i].Name] = &manifest.Plugins[i]
		}
		for i := range manifest.Devices {
			index.devices[manifest.Devices[i].Name] = &manifest.Devices[i]
		}
		index.globals = map[string]*netlabcapsgen.SkyforgeGlobal{}
		for i := range manifest.SkyforgeGlobals {
			index.globals[manifest.SkyforgeGlobals[i].Name] = &manifest.SkyforgeGlobals[i]
		}
	})
}

// SkyforgeGlobal returns the field shape of one Skyforge-only top-level block
// (otg, ...). These are neither modules nor plugins: an author writes them at
// the top of the topology, netlab validates them from the patched attribute
// schema, and nothing in upstream knows they exist. Without them in the
// manifest a question like "what goes in otg:" has no grounded answer.
func SkyforgeGlobal(name string) (*netlabcapsgen.SkyforgeGlobal, bool) {
	load()
	g, ok := index.globals[strings.TrimSpace(name)]
	return g, ok
}

// SkyforgeGlobalNames lists the Skyforge-only top-level blocks, sorted.
func SkyforgeGlobalNames() []string {
	return names(func() []string {
		out := make([]string, 0, len(manifest.SkyforgeGlobals))
		for _, g := range manifest.SkyforgeGlobals {
			out = append(out, g.Name)
		}
		return out
	})
}

// Manifest returns the whole parsed manifest for consumers that need to present
// it rather than query one fact — the netlab_capabilities tool renders
// scopes, attribute types, valid values, the can_be_false/can_be_true idioms and
// per-device feature paths, none of which the narrow accessors expose.
//
// Callers MUST check Loaded() first: an empty manifest knows nothing, and
// "absent from an empty list" is not evidence of absence.
func Manifest() netlabcapsgen.Manifest {
	load()
	return manifest
}

// Loaded reports whether the manifest parsed and is non-empty. A consumer MUST
// check this before treating an unknown name as an error: a manifest that failed
// to load knows nothing, and "not in an empty list" is not evidence of absence.
func Loaded() bool {
	load()
	return len(manifest.Modules) > 0 && len(manifest.Plugins) > 0
}

// ModuleNames returns every netlab module in the patched tree, sorted.
func ModuleNames() []string { return names(func() []string { return moduleNames() }) }

// PluginNames returns every netlab plugin in the patched tree, named the way an
// author writes it in `plugin:` (dotted for the nested ones: firewall.policy,
// ospf.areas, tunnel.gre).
func PluginNames() []string { return names(func() []string { return pluginNames() }) }

// DeviceNames returns every netlab device definition in the patched tree.
func DeviceNames() []string { return names(func() []string { return deviceNames() }) }

func names(f func() []string) []string {
	load()
	out := f()
	sort.Strings(out)
	return out
}

func moduleNames() []string {
	out := make([]string, 0, len(manifest.Modules))
	for _, m := range manifest.Modules {
		out = append(out, m.Name)
	}
	return out
}

func pluginNames() []string {
	out := make([]string, 0, len(manifest.Plugins))
	for _, p := range manifest.Plugins {
		out = append(out, p.Name)
	}
	return out
}

func deviceNames() []string {
	out := make([]string, 0, len(manifest.Devices))
	for _, d := range manifest.Devices {
		out = append(out, d.Name)
	}
	return out
}

// HasModule / HasPlugin / HasDevice answer existence. Each returns false when
// the manifest did not load, so callers must gate on Loaded().
func HasModule(name string) bool { load(); _, ok := index.modules[strings.TrimSpace(name)]; return ok }
func HasPlugin(name string) bool { load(); _, ok := index.plugins[strings.TrimSpace(name)]; return ok }
func HasDevice(name string) bool { load(); _, ok := index.devices[strings.TrimSpace(name)]; return ok }

// PluginAttributes returns the authorable attributes netlab itself declares for a
// plugin (its defaults.yml `attributes:` block): names, types, nesting and
// `_required`. The second return is false when the plugin is unknown or the
// manifest did not load; an empty slice with true means the plugin is real and
// declares none, which is NOT the same statement.
func PluginAttributes(name string) ([]netlabcapsgen.Attribute, bool) {
	load()
	if !Loaded() {
		return nil, false
	}
	p, ok := index.plugins[strings.TrimSpace(name)]
	if !ok {
		return nil, false
	}
	return append([]netlabcapsgen.Attribute(nil), p.Attributes...), true
}

// DeviceModules returns the module names a device declares, UNIONED ALONG THE
// PARENT CHAIN.
//
// The chain is not optional. `iol` has `parent: ios` and its own features block
// lists neither ospf nor mpls — both come from ios. A reader that stopped at the
// device's own file would conclude an IOL router cannot run MPLS, which is how
// "most of the IOS family" gets reported as unsupported.
//
// The second return value is false when the device is not in the manifest. A
// caller must not treat that as "supports nothing": an unknown device is
// unknown, and the difference is the whole reason this returns two values.
func DeviceModules(device string) (map[string]bool, bool) {
	load()
	out := map[string]bool{}
	name := strings.TrimSpace(device)
	seen := map[string]bool{}
	found := false
	for name != "" && !seen[name] {
		seen[name] = true
		d, ok := index.devices[name]
		if !ok {
			break
		}
		found = true
		for _, m := range d.Modules {
			out[m] = true
		}
		name = strings.TrimSpace(d.Parent)
	}
	return out, found
}

// KneServicePorts returns the TCP ports the kne provider exposes on a node of
// this netlab device, from KNE_SERVICE_MAP. The second return is false when the
// device is in neither the service map nor the device list (or the manifest did
// not load): an UNKNOWN device is not a device with no web UI, and a caller must
// not refuse it as one. A known device absent from the map exposes nothing.
// Only the exact device name matches: a parent device is not consulted.
func KneServicePorts(device string) ([]int, bool) {
	load()
	name := strings.ToLower(strings.TrimSpace(device))
	entries, inMap := manifest.KneServices[name]
	if !inMap {
		if len(manifest.KneServices) == 0 {
			return nil, false
		}
		if _, known := index.devices[name]; !known {
			return nil, false
		}
		return []int{}, true
	}
	ports := make([]int, 0, len(entries))
	for _, e := range entries {
		ports = append(ports, e.Port)
	}
	return ports, true
}

// KneServiceExposes reports whether the device's service map carries port. The
// second return is false for an unknown device (see KneServicePorts).
func KneServiceExposes(device string, port int) (exposed, known bool) {
	ports, known := KneServicePorts(device)
	if !known {
		return false, false
	}
	for _, p := range ports {
		if p == port {
			return true, true
		}
	}
	return false, true
}

// HasWebUI reports whether a node of this device serves a web UI through the
// platform's browser proxy: its service map carries the port its web UI listens on
// (WebUIEndpoint: 443 unless web_ui_overrides.json says otherwise, e.g. FortiOS 80).
// The second return is false for an unknown device, where the caller should
// leave the action available rather than hide it on a guess.
func HasWebUI(device string) (hasWebUI, known bool) {
	if WebUIUnsupportedReason(device) != "" {
		return false, true
	}
	_, port := WebUIEndpoint(device)
	return KneServiceExposes(device, port)
}

// ModuleRequires returns the modules netlab REFUSES a topology without. It is
// published so a checker can avoid duplicating an error netlab already raises.
func ModuleRequires(module string) []string {
	load()
	if m, ok := index.modules[strings.TrimSpace(module)]; ok {
		return append([]string(nil), m.Requires...)
	}
	return nil
}

// ValidValues returns the declared enum for `<module>.<path>`, e.g.
// ValidValues("evpn", "transport") -> [vxlan mpls sr]. Empty means netlab
// declares no enum, NOT that any value is valid.
func ValidValues(module, path string) []string {
	load()
	m, ok := index.modules[strings.TrimSpace(module)]
	if !ok {
		return nil
	}
	path = strings.TrimSpace(path)
	for _, a := range m.Attributes {
		if a.Path == path && len(a.ValidValues) > 0 {
			return append([]string(nil), a.ValidValues...)
		}
	}
	return nil
}

// NearestNames returns manifest names close to a misspelling, so an "unknown
// plugin" message can name the right one instead of only the wrong one.
// Matching is deliberately crude — shared prefix before the dot, or one name
// containing the other — because the failures it exists for are of that shape
// (tunnel.dmvpn -> tunnel.gre, ospf.instances -> ospf.areas, bgp.rr_group ->
// bgp.domain/bgp.policy/bgp.session).
func NearestNames(want string, candidates []string) []string {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return nil
	}
	prefix := want
	if i := strings.Index(want, "."); i > 0 {
		prefix = want[:i]
	}
	out := []string{}
	for _, c := range candidates {
		lc := strings.ToLower(c)
		switch {
		case strings.HasPrefix(lc, prefix+"."), lc == prefix:
			out = append(out, c)
		case strings.Contains(lc, want), strings.Contains(want, lc):
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// FactNetlabCapabilities is the fact key this package answers for a fact-serving
// skill corpus. It is a SEPARATE AXIS from the netlab.devices / netlab.modules /
// netlab.plugins keys that corpus already serves: those say what each DEVICE
// renders, this says WHAT EXISTS AT ALL in the patched netlab, which is the
// question a name typo fails.
const FactNetlabCapabilities = "netlab.capabilities"

// Fact answers FactNetlabCapabilities and nothing else, in the shape
// a fact function expects — the caller and this package do not import each
// other, so the integration is one assignment at the service seam.
//
// It returns false when the manifest did not load, never an empty string. An
// unreadable manifest and a platform with no capabilities are different states,
// and rendering them the same way is how a skill starts asserting that a
// capability does not exist.
func Fact(key string) (string, bool) {
	if key != FactNetlabCapabilities {
		return "", false
	}
	load()
	if !Loaded() {
		return "", false
	}
	return capabilityCatalogue(), true
}

// NO WORKED EXAMPLES LIVE HERE, and one used to. This rendering asserted that
// `tunnel.dmvpn`, `ospf.instances` and `bgp.rr_group` do not exist, citing the
// example topology that named all three. The patch series has since ADDED all
// three -- the manifest now carries 32 plugins where it carried 29 -- so the
// sentence became a generated artifact confidently telling a reader that three
// real plugins were imaginary. A hand-written example inside a generated
// rendering cannot be regenerated, so it survives the fact it described.
//
// capabilityCatalogue is the rendered statement of the authorable names.
// It is names only, on purpose: the full manifest is 65 KB, and the failure it
// prevents — naming a module or plugin that does not exist — needs nothing more
// than the list.
func capabilityCatalogue() string {
	var b strings.Builder
	b.WriteString("Generated from the PATCHED netlab tree (" + catalogueSeriesNote + " over upstream), so upstream's documentation is NOT ")
	b.WriteString("what this platform accepts. Author ONLY names from these lists; anything else fails the real compile, and the ")
	b.WriteString("authoring preflight now flags it first.\n\n")
	b.WriteString("**Modules** (`module:`): " + strings.Join(moduleNamesSorted(), ", ") + "\n\n")
	b.WriteString("**Plugins** (`plugin:`), written exactly as shown, dots included: " + strings.Join(pluginNamesSorted(), ", ") + "\n\n")
	b.WriteString(missingCapabilityAdvice)
	if vv := ValidValues("evpn", "transport"); len(vv) > 0 {
		b.WriteString("Enums netlab declares and people get wrong: `evpn.transport` is one of " + strings.Join(vv, "/") +
			" and the matching module must be loaded; `sr.protocol` is one of " + strings.Join(ValidValues("sr", "protocol"), "/") + ".\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func moduleNamesSorted() []string { n := moduleNames(); sort.Strings(n); return n }
func pluginNamesSorted() []string { n := pluginNames(); sort.Strings(n); return n }
