package intentrules

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
	"gopkg.in/yaml.v3"
)

// This file turns a netlab topology.yml, as the designer and validate see it, into the objects of the schema. It is a second,
// small model of what netlab does with an authored document, so it models only what the rules read, follows netlab's own
// inheritance (node, then group, then topology) and fills the defaults netlab fills, and says so (Graph.Skipped) when it meets
// something it cannot resolve. netlab remains the authority: where it prints a finding for the same code, that one wins.

// nodeGeneratingPlugins create nodes at compile time, so a link may name a node the document does not define.
var nodeGeneratingPlugins = map[string]bool{"fabric": true, "node.clone": true, "multilab": true, "multiserver": true}

type loadedNode struct {
	name     string
	data     *yaml.Node // mapping, or nil
	path     string
	obj      *Object
	modules  []string        // own module list
	restrict map[string]bool // VLANs the node carries over VXLAN when evpn.vlans / vxlan.vlans limit them; nil when they do not
	asn      string          // effective bgp.as (node, then a group, then the topology); "" when none is set
	hasBGP   bool
	hasOwn   bool
	device   string
	role     string
}

// Load builds the graph for a document. root is the document's top-level mapping.
func Load(s *Schema, root *yaml.Node) *Graph {
	g := NewGraph()
	root = deref(root)
	if !isMap(root) {
		g.Skip("document", "the document is not a YAML mapping")
		return g
	}
	prefix := ""
	if w := get(root, "topology"); isMap(w) && get(root, "nodes") == nil && get(root, "links") == nil {
		root, prefix = w, "topology."
	}
	l := &loader{s: s, g: g, root: root, prefix: prefix}
	l.run()
	return g
}

type loader struct {
	s      *Schema
	g      *Graph
	root   *yaml.Node
	prefix string

	topo        *Object
	nodes       map[string]*loadedNode
	nodeOrder   []string
	groups      map[string]*yaml.Node
	groupOrder  []string
	groupObjs   map[string]*Object
	vlans       map[string]*Object
	modules     map[string]*Object
	plugins     map[string]*Object
	topoMods    []string
	hasTopoMod  bool
	defDevice   string
	genNodes    bool
	prefixes    []string // include prefixes
	known       map[string]bool
	placeholder map[string]*Object
	vlanUsers   map[string]map[string]bool // vlan -> node set
	vlanNoSVI   map[string]map[string]bool // vlan -> nodes whose interface for it is routed (vlan.mode on the link or interface): no SVI there
}

func (l *loader) run() {
	l.topo = l.g.Add(&Object{Kind: "NetlabTopology", Key: "topology", Path: strings.TrimSuffix(l.prefix, "."),
		Attrs: map[string]any{"name": scalar(get(l.root, "name"))}})
	l.defDevice = scalar(getPath(l.root, "defaults.device"))
	if l.defDevice == "" {
		l.defDevice = scalar(get(l.root, "device"))
	}
	if m := get(l.root, "module"); m != nil {
		l.hasTopoMod = true
		l.topoMods = stringList(m)
	}
	if hasInclude(l.root) {
		// netlab splices the included files into the document before it reads it, so nodes, groups and VLANs may be defined there
		l.genNodes = true
		l.g.Skip("includes", "the document includes other files (_include); what they define is not visible here, so references and group membership are not judged")
	}
	l.loadPlugins()
	l.loadModules()
	l.loadNodes()
	l.autoCreate()
	l.loadGroups()
	l.resolveNodes()
	l.loadVlans()
	l.loadLinks()
	l.attachVlanUsers()
}

func (l *loader) path(p string) string { return l.prefix + p }

// ---- plugins ---------------------------------------------------------------------------------------------------------------

func (l *loader) loadPlugins() {
	l.plugins = map[string]*Object{}
	enabled := stringList(get(l.root, "plugin"))
	for _, name := range enabled {
		if _, ok := l.plugins[name]; ok {
			continue
		}
		o := l.g.Add(&Object{Kind: "NetlabPlugin", Key: "plugin:" + name, Path: l.path("plugin"), Attrs: map[string]any{"name": name}})
		o.Link("topology", l.topo)
		l.plugins[name] = o
		// a plugin that creates nodes, or one this build does not know (a local plugin can create anything)
		if nodeGeneratingPlugins[name] || !netlabcaps.HasPlugin(name) {
			l.genNodes = true
		}
	}
	// the top-level keys that are a plugin's configuration block, read from netlab's own plugin definitions
	owners := pluginBlockOwners()
	for _, key := range keysOf(l.root) {
		owner, ok := owners[key]
		if !ok {
			continue
		}
		o := l.g.Add(&Object{Kind: "NetlabPluginBlock", Key: "pluginblock:" + key, Path: l.path(key), Attrs: map[string]any{"key": key, "owner": owner}})
		o.Link("topology", l.topo)
		if p, on := l.plugins[owner]; on {
			o.Link("plugin", p)
		}
	}
}

var (
	ownersOnce  sync.Once
	ownersCache map[string]string
)

// pluginBlockOwners maps a top-level topology key to the netlab plugin whose defaults declare it as a global attribute.
func pluginBlockOwners() map[string]string {
	ownersOnce.Do(func() {
		out := map[string]string{}
		for _, p := range netlabcaps.Manifest().Plugins {
			for _, a := range p.Attributes {
				if a.Scope != "global" {
					continue
				}
				key := strings.SplitN(strings.SplitN(a.Path, ".", 2)[0], "[", 2)[0]
				if key != "" {
					if _, taken := out[key]; !taken {
						out[key] = p.Name
					}
				}
			}
		}
		ownersCache = out
	})
	return ownersCache
}

// ---- modules ---------------------------------------------------------------------------------------------------------------

func (l *loader) loadModules() {
	l.modules = map[string]*Object{}
	for _, name := range netlabcaps.ModuleNames() {
		l.moduleObj(name)
	}
	for _, name := range sortedKeys(l.modules) {
		for _, req := range netlabcaps.ModuleRequires(name) {
			l.modules[name].Link("requires", l.moduleObj(req))
		}
	}
}

func (l *loader) moduleObj(name string) *Object {
	if m, ok := l.modules[name]; ok {
		return m
	}
	m := l.g.Add(&Object{Kind: "NetlabModule", Key: "module:" + name, Path: l.path("module"), Attrs: map[string]any{"name": name}})
	m.Link("topology", l.topo)
	l.modules[name] = m
	return m
}

// ---- nodes -----------------------------------------------------------------------------------------------------------------

func (l *loader) loadNodes() {
	l.nodes = map[string]*loadedNode{}
	l.known = map[string]bool{}
	n := get(l.root, "nodes")
	add := func(name string, data *yaml.Node, path string) {
		ln := &loadedNode{name: name, path: path}
		if isMap(data) {
			ln.data = data
			if i := get(data, "include"); i != nil {
				l.prefixes = append(l.prefixes, name+"_")
			}
			if m := get(data, "module"); m != nil {
				ln.hasOwn, ln.modules = true, stringList(m)
			}
			ln.device = scalar(get(data, "device"))
			ln.role = scalar(get(data, "role"))
		}
		o := l.g.Add(&Object{Kind: "NetlabDevice", Key: "node:" + name, Path: path, Attrs: map[string]any{"name": name}})
		o.Link("topology", l.topo)
		ln.obj = o
		l.known[name] = true
		if _, dup := l.nodes[name]; !dup {
			l.nodes[name] = ln
			l.nodeOrder = append(l.nodeOrder, name)
		} else {
			// a second definition of the same name: the object exists for the uniqueness rule; the first one stays the one the other rules read
			l.nodeOrder = append(l.nodeOrder, name+"\x00dup")
		}
	}
	switch {
	case isMap(n):
		for _, p := range mapPairs(n) {
			add(p[0].Value, p[1], l.path("nodes."+p[0].Value))
		}
		// YAML lets a key repeat; a loader that builds a map keeps the last. mapPairs keeps both, so the repeat is seen.
	case isSeq(n):
		for i, item := range deref(n).Content {
			item = deref(item)
			switch {
			case isScalar(item) && scalar(item) != "":
				add(scalar(item), nil, l.path(fmt.Sprintf("nodes[%d]", i)))
			case isMap(item) && scalar(get(item, "name")) != "":
				add(scalar(get(item, "name")), item, l.path(fmt.Sprintf("nodes[%d]", i)))
			}
		}
	}
}

// autoCreate adds the nodes netlab creates from group members: groups._auto_create (or the key on one group) makes every listed
// member that is neither a node nor a group into a node.
func (l *loader) autoCreate() {
	g := get(l.root, "groups")
	if !isMap(g) {
		return
	}
	all := isTrue(get(g, "_auto_create")) || isTrue(getPath(l.root, "defaults.groups._auto_create"))
	for _, p := range mapPairs(g) {
		name := p[0].Value
		if strings.HasPrefix(name, "_") {
			continue
		}
		members := stringList(p[1]) // a group written as a list is its members list
		if isMap(p[1]) {
			if !all && !isTrue(get(p[1], "_auto_create")) {
				continue
			}
			if t := scalar(get(p[1], "type")); t != "" && t != "node" {
				continue
			}
			members = stringList(get(p[1], "members"))
		} else if !isSeq(p[1]) || !all {
			continue
		}
		for _, m := range members {
			if patternRE.MatchString(m) || strings.HasPrefix(m, "~") {
				continue // a pattern selects existing nodes, it does not create one
			}
			if get(g, m) != nil || l.known[m] {
				continue
			}
			o := l.g.Add(&Object{Kind: "NetlabDevice", Key: "node:" + m, Path: l.path("groups." + name + ".members"), Attrs: map[string]any{"name": m}})
			o.Link("topology", l.topo)
			l.nodes[m] = &loadedNode{name: m, path: o.Path, obj: o}
			l.nodeOrder = append(l.nodeOrder, m)
			l.known[m] = true
		}
	}
}

// ---- groups ----------------------------------------------------------------------------------------------------------------

func (l *loader) loadGroups() {
	l.groups = map[string]*yaml.Node{}
	l.groupObjs = map[string]*Object{}
	g := get(l.root, "groups")
	if !isMap(g) {
		return
	}
	for _, p := range mapPairs(g) {
		name := p[0].Value
		data := deref(p[1])
		if strings.HasPrefix(name, "_") {
			continue // global settings such as _auto_create, not groups
		}
		if _, dup := l.groups[name]; dup {
			continue
		}
		if t := scalar(get(data, "type")); isMap(data) && t != "" && t != "node" {
			l.g.Skip("groups", "a group of type "+t+" is not modelled")
			continue
		}
		l.groups[name] = data
		l.groupOrder = append(l.groupOrder, name)
		o := l.g.Add(&Object{Kind: "NetlabGroup", Key: "group:" + name, Path: l.path("groups." + name),
			Attrs: map[string]any{"name": name, "populated_by_plugin": l.genNodes || name == "all" || netlabcaps.HasDevice(name) || hasPattern(data) || l.hasUnknownMember(data), "has_module_list": isMap(data) && get(data, "module") != nil}})
		o.Link("topology", l.topo)
		l.groupObjs[name] = o
	}
}

// members returns the node names in a group, directly or through nested groups. A cycle is cut by the visited set (netlab reports a
// recursive group itself), and a member written as a pattern (~regex or a glob) is expanded against the node names.
func (l *loader) members(name string, seen map[string]bool) []string {
	if seen == nil {
		seen = map[string]bool{}
	}
	if seen[name] {
		return nil
	}
	seen[name] = true
	data := l.groups[name]
	var out []string
	add := func(n string) {
		if _, isGroup := l.groups[n]; isGroup {
			out = append(out, l.members(n, seen)...)
			return
		}
		if l.known[n] {
			out = append(out, n)
			return
		}
		out = append(out, l.matchNodes(n)...)
	}
	switch {
	case isSeq(data):
		for _, m := range stringList(data) {
			add(m)
		}
	case isMap(data):
		if t := scalar(get(data, "type")); t != "" && t != "node" {
			return nil
		}
		for _, m := range stringList(get(data, "members")) {
			add(m)
		}
	}
	if name == "all" {
		out = append(out, l.nodeNames()...)
	}
	// nodes that name the group
	for _, nn := range l.nodeOrder {
		ln := l.nodes[nn]
		if ln == nil || ln.data == nil {
			continue
		}
		for _, gn := range stringList(get(ln.data, "group")) {
			if gn == name {
				out = append(out, ln.name)
			}
		}
	}
	// a group named like a device holds every node of that device, whatever else it says
	if name != "all" && netlabcaps.HasDevice(name) {
		for _, nn := range l.nodeNames() {
			if l.nodes[nn].device == name {
				out = append(out, nn)
			}
		}
	}
	return out
}

var patternRE = regexp.MustCompile(`[\[\].*?!]`)

// matchNodes expands a group member written as a pattern (~regex, or a glob) into the node names it matches. A name that is
// neither returns nothing.
func (l *loader) matchNodes(pattern string) []string {
	var out []string
	switch {
	case strings.HasPrefix(pattern, "~"):
		re, err := regexp.Compile("^(?:" + strings.TrimPrefix(pattern, "~") + ")")
		if err != nil {
			return nil
		}
		for _, n := range l.nodeNames() {
			if re.MatchString(n) {
				out = append(out, n)
			}
		}
	case patternRE.MatchString(pattern):
		glob := strings.ReplaceAll(pattern, "[!", "[^")
		for _, n := range l.nodeNames() {
			if ok, err := path.Match(glob, n); err == nil && ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// hasUnknownMember reports whether a group lists a name that is neither a node nor a group. netlab refuses that itself ("Member X of
// group Y is not a valid node or group name"), so the group is not also reported as empty.
func (l *loader) hasUnknownMember(data *yaml.Node) bool {
	var list []string
	if isSeq(data) {
		list = stringList(data)
	} else if isMap(data) {
		list = stringList(get(data, "members"))
	}
	for _, m := range list {
		if strings.HasPrefix(m, "~") || patternRE.MatchString(m) {
			continue
		}
		if _, isGroup := l.groups[m]; !isGroup && !l.known[m] {
			return true
		}
	}
	return false
}

// hasPattern reports whether a group lists a member as a pattern, which this layer expands only approximately.
func hasPattern(data *yaml.Node) bool {
	var list []string
	if isSeq(data) {
		list = stringList(data)
	} else if isMap(data) {
		list = stringList(get(data, "members"))
	}
	for _, m := range list {
		if strings.HasPrefix(m, "~") || patternRE.MatchString(m) {
			return true
		}
	}
	return false
}

// hasInclude reports whether any mapping in the document carries an _include key.
func hasInclude(n *yaml.Node) bool {
	n = deref(n)
	if n == nil {
		return false
	}
	switch n.Kind {
	case yaml.MappingNode:
		for _, p := range mapPairs(n) {
			if p[0].Value == "_include" || hasInclude(p[1]) {
				return true
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if hasInclude(c) {
				return true
			}
		}
	}
	return false
}

func (l *loader) nodeNames() []string {
	var out []string
	for _, n := range l.nodeOrder {
		if !strings.Contains(n, "\x00") {
			out = append(out, n)
		}
	}
	return out
}

// resolveNodes fills each node's device and module set the way netlab does, and links node to group and module objects.
func (l *loader) resolveNodes() {
	memberOf := map[string][]string{} // node -> groups
	for _, gn := range l.groupOrder {
		seen := map[string]bool{}
		for _, m := range l.members(gn, nil) {
			if !seen[m] {
				seen[m] = true
				memberOf[m] = append(memberOf[m], gn)
			}
		}
	}
	for _, name := range l.nodeNames() {
		ln := l.nodes[name]
		groups := memberOf[name]
		// device: node, then group, then defaults.device
		dev := ln.device
		if dev == "" {
			for _, gn := range groups {
				if d := scalar(get(l.groups[gn], "device")); d != "" {
					dev = d
					break
				}
			}
		}
		if dev == "" {
			dev = l.defDevice
		}
		ln.device = dev
		ln.obj.Attrs["device"] = dev
		// the autonomous system: the node's own, else the first group that sets one, else the topology's
		ln.asn = scalar(getPath(ln.data, "bgp.as"))
		for _, gn := range groups {
			if ln.asn != "" {
				break
			}
			ln.asn = scalar(getPath(l.groups[gn], "bgp.as"))
		}
		if ln.asn == "" {
			ln.asn = scalar(getPath(l.root, "bgp.as"))
		}
		// a vlan.mode on the node or on a group it is in decides every VLAN on the node: bridge and route have no SVI
		ln.obj.Attrs["svi_possible"] = true
		for _, src := range append([]*yaml.Node{ln.data}, groupNodes(l, groups)...) {
			if m := scalar(getPath(src, "vlan.mode")); m == "bridge" || m == "route" {
				ln.obj.Attrs["svi_possible"] = false
			}
		}
		ln.restrict = vlanRestriction(append([]*yaml.Node{ln.data}, groupNodes(l, groups)...))
		// modules, as the pinned netlab resolves them (measured, not read): the node's own list, else the topology list (never for a
		// host); then every module list of every group the node is in is ADDED.
		isHost := ln.role == "host" || dev == "linux"
		var eff []string
		switch {
		case ln.hasOwn:
			eff = append(eff, ln.modules...)
		case l.hasTopoMod && !isHost:
			eff = append(eff, l.topoMods...)
		}
		for _, gn := range groups {
			if gn == "all" && isHost {
				continue
			}
			eff = append(eff, stringList(get(l.groups[gn], "module"))...)
		}
		supported, known := netlabcaps.DeviceModules(dev)
		seen := map[string]bool{}
		for _, m := range eff {
			if seen[m] || m == "" {
				continue
			}
			if known && !supported[m] {
				continue // the device cannot run it; netlab reports that itself
			}
			seen[m] = true
			ln.obj.Link("modules", l.moduleObj(m))
		}
		// modules a module of the node requires and the node lacks (netlab: "Module evpn on device eos requires bgp module")
		have := map[string]bool{}
		for _, m := range eff {
			if !known || supported[m] {
				have[m] = true
			}
		}
		ln.hasBGP = have["bgp"]
		for _, m := range sortedKeys(have) {
			for _, req := range netlabcaps.ModuleRequires(m) {
				if !have[req] && (!known || supported[req]) {
					ln.obj.Link("missing_modules", l.moduleObj(req))
				}
			}
		}
		for _, gn := range groups {
			if l.groupObjs[gn] == nil {
				continue
			}
			ln.obj.Link("groups", l.groupObjs[gn])
			l.groupObjs[gn].Link("members", ln.obj)
		}
	}
	for _, gn := range l.groupOrder {
		for _, m := range stringList(get(l.groups[gn], "module")) {
			l.groupObjs[gn].Link("modules", l.moduleObj(m))
		}
	}
}

// ---- vlans -----------------------------------------------------------------------------------------------------------------

func (l *loader) loadVlans() {
	l.vlans = map[string]*Object{}
	l.vlanUsers = map[string]map[string]bool{}
	v := get(l.root, "vlans")
	if !isMap(v) {
		return
	}
	// the default mode: the topology's own vlan.mode, else defaults.vlan.mode, else irb (netlab's vlan module default)
	defMode := scalar(getPath(l.root, "vlan.mode"))
	if defMode == "" {
		defMode = scalar(getPath(l.root, "defaults.vlan.mode"))
	}
	if defMode == "" {
		defMode = "irb"
	}
	for _, p := range mapPairs(v) {
		name := p[0].Value
		data := deref(p[1])
		mode := defMode
		svi := "on"
		inVRF := false
		if isMap(data) {
			if m := scalar(get(data, "mode")); m != "" {
				mode = m
			}
			if o := getPath(data, "ospf"); o != nil {
				switch {
				case isFalse(o):
					svi = "off"
				case isMap(o) && isTrue(getPath(o, "passive")):
					svi = "passive"
				}
			}
			if p := getPath(data, "ospf.passive"); p != nil && isTrue(p) {
				svi = "passive"
			}
			// an external VLAN (role: external) carries no IGP: netlab leaves routing protocols off external links
			if scalar(get(data, "role")) == "external" {
				svi = "off"
			}
			inVRF = get(data, "vrf") != nil
			// No address, no routing protocol on the SVI: the l2only pool has none, and a prefix with ipv4 turned off (the measured
			// forms; `prefix: false` alone is ignored by netlab, which still addresses the SVI).
			if scalar(get(data, "pool")) == "l2only" {
				svi = "off"
			}
			if pf := get(data, "prefix"); isMap(pf) && isFalse(get(pf, "ipv4")) {
				svi = "off"
			}
			// A pool that carries neither an IPv4 nor an IPv6 prefix addresses nothing, whatever its name (measured: the SVI gets no
			// address and no OSPF). A pool with IPv6 only still runs OSPFv3 on the SVI (measured), so it is not suppressed.
			if pool := scalar(get(data, "pool")); pool != "" && !l.poolAddresses(pool) {
				svi = "off"
			}
		}
		o := l.g.Add(&Object{Kind: "NetlabVlan", Key: "vlan:" + name, Path: l.path("vlans." + name),
			Attrs: map[string]any{"name": name, "mode": mode, "svi_ospf": svi, "in_vrf": inVRF,
				"svi_ospf_warning": !isFalse(getPath(l.root, "defaults.evpn.warnings.svi_ospf"))}})
		o.Link("topology", l.topo)
		if _, dup := l.vlans[name]; !dup {
			l.vlans[name] = o
			l.vlanUsers[name] = map[string]bool{}
		}
		if isMap(data) {
			for _, ref := range vlanLinkNodes(get(data, "links")) {
				l.vlanUsers[name][ref] = true
			}
		}
	}
}

// builtinAddressPools are the pools netlab defines itself (netsim/defaults/addressing.yml) and whether each has an address family
// by default; l2only has none. A topology's own addressing: (or defaults.addressing:) merges key by key over them.
var builtinAddressPools = map[string]bool{"loopback": true, "router_id": true, "lan": true, "p2p": true, "mgmt": true, "vrf_loopback": true, "l2only": false}

// poolAddresses reports whether the named address pool hands out an IPv4 or IPv6 prefix. A pool the document does not define and
// netlab does not either is unknown here (netlab reports it itself), so it is taken to address.
func (l *loader) poolAddresses(name string) bool {
	has, ok := builtinAddressPools[name]
	family := func(def *yaml.Node, key string) (set, on bool) {
		v := get(def, key)
		if v == nil {
			return false, false
		}
		return true, !isFalse(v) && scalar(v) != ""
	}
	v4, v6 := has, has
	for _, src := range []*yaml.Node{get(l.root, "addressing"), getPath(l.root, "defaults.addressing")} {
		def := get(src, name)
		if def == nil || !isMap(def) {
			continue
		}
		ok = true
		if set, on := family(def, "ipv4"); set {
			v4 = on
		}
		if set, on := family(def, "ipv6"); set {
			v6 = on
		}
	}
	if !ok {
		return true
	}
	return v4 || v6
}

// vlanLinkNodes reads the node names out of a vlans.<name>.links list, whatever form its items take.
func vlanLinkNodes(n *yaml.Node) []string {
	var out []string
	for _, item := range seqItems(n) {
		item = deref(item)
		switch {
		case isScalar(item):
			for _, part := range strings.FieldsFunc(scalar(item), func(r rune) bool { return r == '-' || r == ':' }) {
				out = append(out, strings.TrimSpace(part))
			}
		case isSeq(item):
			for _, c := range item.Content {
				out = append(out, scalar(c))
			}
		case isMap(item):
			for _, k := range keysOf(item) {
				out = append(out, k)
			}
		}
	}
	return out
}

// markMixedAS sets Vlan.mixed_as: BGP gives a link between nodes of different autonomous systems the role external, and OSPF does
// not run on an external link, so a VLAN whose BGP speakers are not all in one AS gets no OSPF on its SVIs.
func (l *loader) markMixedAS() {
	for _, vn := range sortedKeys(l.vlans) {
		seen := map[string]bool{}
		for _, node := range sortedKeys(l.vlanUsers[vn]) {
			if ln := l.nodes[node]; ln != nil && ln.hasBGP {
				seen[ln.asn] = true
			}
		}
		l.vlans[vn].Attrs["mixed_as"] = len(seen) > 1
	}
}

func (l *loader) attachVlanUsers() {
	// node-level vlans: block
	for _, name := range l.nodeNames() {
		ln := l.nodes[name]
		if ln.data == nil {
			continue
		}
		l.useVlansBlock(get(ln.data, "vlans"), name)
	}
	// a group's vlans: block reaches every member
	for _, gn := range l.groupOrder {
		if isMap(l.groups[gn]) {
			if vb := get(l.groups[gn], "vlans"); vb != nil {
				for _, m := range l.members(gn, nil) {
					l.useVlansBlock(vb, m)
				}
			}
		}
	}
	l.markMixedAS()
	for _, vn := range sortedKeys(l.vlans) {
		for _, node := range sortedKeys(l.vlanUsers[vn]) {
			if ln := l.nodes[node]; ln != nil {
				if ln.restrict != nil && !ln.restrict[vn] {
					continue // the node limits which VLANs ride VXLAN and this one is not listed
				}
				if l.vlanNoSVI[vn][node] {
					continue // an interface of the node carries the VLAN routed (vlan.mode on the link or the interface): no SVI to run OSPF on
				}
				l.vlans[vn].Link("devices", ln.obj)
				ln.obj.Link("vlans", l.vlans[vn])
			}
		}
	}
}

func (l *loader) useVlansBlock(nv *yaml.Node, node string) {
	switch {
	case isMap(nv):
		for _, k := range keysOf(nv) {
			l.useVlan(k, node)
		}
	case isSeq(nv):
		for _, k := range stringList(nv) {
			l.useVlan(k, node)
		}
	}
}

func (l *loader) useVlan(vlan, node string) {
	if _, ok := l.vlans[vlan]; ok {
		l.vlanUsers[vlan][node] = true
	}
}

// ---- links -----------------------------------------------------------------------------------------------------------------

type endpoint struct {
	node string
	data *yaml.Node
}

func (l *loader) validNodeName(n string) bool {
	if l.known[n] {
		return true
	}
	for _, p := range l.prefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

func (l *loader) loadLinks() {
	links := get(l.root, "links")
	if links == nil {
		return
	}
	if isMap(links) {
		l.g.Skip("links", "links is written as groups of links; their endpoints are not expanded here")
		return
	}
	if !isSeq(links) {
		return
	}
	// A reference to a node the document does not define can only be judged when nothing else can create nodes.
	canJudgeRefs := !l.genNodes
	if !canJudgeRefs {
		l.g.Skip("link-unknown-node", "a node-generating plugin is enabled, so a link may name a node the document does not define")
	}
	linkSeen := 0
	for i, item := range deref(links).Content {
		item = deref(item)
		path := l.path(fmt.Sprintf("links[%d]", i))
		eps, attrs, ok := l.parseLink(item)
		if !ok {
			l.g.Skip("links", "a link written in a form this layer does not expand was not judged")
			continue
		}
		if isTrue(attrs["disable"]) {
			continue
		}
		typ := scalar(attrs["type"])
		if len(eps) == 0 && typ == "" {
			// Links that name their nodes elsewhere (a lag: members block, a link group) have no endpoints here; a link with
			// none at all is netlab's "No valid nodes" error. Either way there is nothing for this layer to count.
			l.g.Skip("links", "a link that names no node directly (lag members, link groups) was not judged")
			continue
		}
		linkSeen++
		var kind string
		switch typ {
		case "p2p":
			kind = "NetlabP2pLink"
		case "lan":
			kind = "NetlabLanLink"
		case "stub", "loopback":
			kind = "NetlabStubLink"
		case "":
			// The kind netlab gives it. A link naming one node is netlab's stub network (`links: [ r1, r1-r2 ]` is the documented
			// way to get one), so a single end without a type is not a missing end.
			switch {
			case len(eps) == 1:
				kind = "NetlabStubLink"
			case len(eps) > 2:
				kind = "NetlabLanLink"
			default:
				kind = "NetlabP2pLink"
			}
		default:
			l.g.Skip("links", fmt.Sprintf("a link of type %s is not modelled", typ))
			continue
		}
		lo := l.g.Add(&Object{Kind: kind, Key: fmt.Sprintf("link:%d", i), Path: path,
			Attrs: map[string]any{"name": fmt.Sprintf("links[%d]", i+1), "link_type": typ}})
		if id := scalar(attrs["linkid"]); id != "" {
			lo.Attrs["linkid"] = id
		}
		lo.Link("topology", l.topo)
		for j, ep := range eps {
			io := l.g.Add(&Object{Kind: "NetlabInterface", Key: fmt.Sprintf("link:%d:ep:%d", i, j), Path: path,
				Attrs: map[string]any{"node": ep.node}})
			if ln := l.nodes[ep.node]; ln != nil {
				io.Link("device", ln.obj)
				l.noteVlans(ep, attrs, ln.name)
			} else if l.validNodeName(ep.node) || !canJudgeRefs {
				io.Link("device", l.placeholderDevice(ep.node))
			} else {
				io.Dangling("device", ep.node)
			}
			lo.Link("endpoints", io)
		}
	}
}

// placeholderDevice stands in for a node a plugin or an include will create, so the endpoint counts and the reference is not
// reported as dangling. It has no modules, so no rule reads it.
func (l *loader) placeholderDevice(name string) *Object {
	if l.placeholder == nil {
		l.placeholder = map[string]*Object{}
	}
	if o, ok := l.placeholder[name]; ok {
		return o
	}
	o := l.g.Add(&Object{Kind: "NetlabDevice", Key: "generated:" + name, Path: l.path("nodes"), Attrs: map[string]any{"name": name + " (generated)"}})
	o.Link("topology", l.topo)
	l.placeholder[name] = o
	return o
}

// parseLink expands one link item the way netlab's adjust_link_object does. attrs holds the link-level attributes.
func (l *loader) parseLink(item *yaml.Node) (eps []endpoint, attrs map[string]*yaml.Node, ok bool) {
	attrs = map[string]*yaml.Node{}
	switch {
	case isScalar(item):
		s := scalar(item)
		if s == "" {
			return nil, attrs, false
		}
		for _, part := range strings.Split(s, "-") {
			eps = append(eps, endpoint{node: strings.TrimSpace(part)})
		}
		return eps, attrs, true
	case isSeq(item):
		for _, c := range deref(item).Content {
			c = deref(c)
			switch {
			case isScalar(c):
				eps = append(eps, endpoint{node: scalar(c)})
			case isMap(c) && scalar(get(c, "node")) != "":
				eps = append(eps, endpoint{node: scalar(get(c, "node")), data: c})
			default:
				return nil, attrs, false
			}
		}
		return eps, attrs, true
	case isMap(item):
		if get(item, "members") != nil || get(item, "group") != nil {
			return nil, attrs, false // a link group: expanded from its members by netlab
		}
		if iface := get(item, "interfaces"); iface != nil {
			for _, p := range mapPairs(item) {
				if p[0].Value != "interfaces" {
					attrs[p[0].Value] = deref(p[1])
				}
			}
			e2, _, ok := l.parseLink(iface)
			if !ok || !isSeq(iface) {
				return nil, attrs, false
			}
			return e2, attrs, true
		}
		for _, p := range mapPairs(item) {
			k := p[0].Value
			if l.validNodeName(k) {
				v := deref(p[1])
				eps = append(eps, endpoint{node: k, data: v})
				continue
			}
			// With a node-generating plugin enabled the node set is not known, so a key that is not a link attribute may be a
			// node the plugin creates: the link cannot be judged.
			if l.genNodes && !linkAttributeName(k) && (isMap(p[1]) || isNull(p[1])) {
				return nil, attrs, false
			}
			attrs[k] = deref(p[1])
		}
		return eps, attrs, true
	}
	return nil, attrs, false
}

// noteVlans records which VLANs a node attaches to through this end of a link: vlan.access, vlan.native and vlan.trunk, written
// on the link or on the end. A vlan.mode written on the end (else on the link) decides the forwarding mode of the VLANs that end
// carries, and so does a mode inside a vlan.trunk entry: route (netlab accepts only route there; bridge is read too) gives that
// node's interface no SVI for the VLAN (measured: the other node's SVI is then the only one and is passive), recorded in vlanNoSVI.
func (l *loader) noteVlans(ep endpoint, linkAttrs map[string]*yaml.Node, node string) {
	type attach struct {
		names []string
		mode  string            // the vlan.mode written beside the names ("" when none)
		trunk map[string]string // a trunk dictionary's own mode per VLAN
	}
	read := func(n *yaml.Node) attach {
		var a attach
		if !isMap(n) {
			return a
		}
		for _, key := range []string{"vlan.access", "vlan.native"} {
			if v := getPath(n, key); v != nil {
				a.names = append(a.names, scalar(v))
			}
		}
		if t := getPath(n, "vlan.trunk"); t != nil {
			switch {
			case isMap(t):
				for _, p := range mapPairs(t) {
					a.names = append(a.names, p[0].Value)
					if m := scalar(getPath(deref(p[1]), "vlan.mode")); m != "" {
						if a.trunk == nil {
							a.trunk = map[string]string{}
						}
						a.trunk[p[0].Value] = m
					}
				}
			default:
				a.names = append(a.names, stringList(t)...)
			}
		}
		a.mode = scalar(getPath(n, "vlan.mode"))
		return a
	}
	// link-level attributes arrive as a mapping of their own keys
	synthetic := &yaml.Node{Kind: yaml.MappingNode}
	keys := make([]string, 0, len(linkAttrs))
	for k := range linkAttrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		synthetic.Content = append(synthetic.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, linkAttrs[k])
	}
	onLink, onEnd := read(synthetic), read(ep.data)
	mode := onEnd.mode
	if mode == "" {
		mode = onLink.mode
	}
	for _, a := range []attach{onLink, onEnd} {
		for _, name := range a.names {
			l.useVlan(name, node)
			m := mode
			if tm := a.trunk[name]; tm != "" {
				m = tm
			}
			if m == "route" || m == "bridge" {
				l.noSVI(name, node)
			}
		}
	}
}

// noSVI records that the node has no SVI for the VLAN through some interface of its own.
func (l *loader) noSVI(vlan, node string) {
	if _, ok := l.vlans[vlan]; !ok {
		return
	}
	if l.vlanNoSVI == nil {
		l.vlanNoSVI = map[string]map[string]bool{}
	}
	if l.vlanNoSVI[vlan] == nil {
		l.vlanNoSVI[vlan] = map[string]bool{}
	}
	l.vlanNoSVI[vlan][node] = true
}

func seqItems(n *yaml.Node) []*yaml.Node {
	n = deref(n)
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil
	}
	return n.Content
}

var (
	linkAttrOnce sync.Once
	linkAttrs    map[string]bool
)

// linkAttributeName reports whether a key of a link mapping is a link or interface attribute netlab knows (core, module or plugin),
// as opposed to a node name. It is read from the capability manifest, not listed here.
func linkAttributeName(key string) bool {
	linkAttrOnce.Do(func() {
		linkAttrs = map[string]bool{"type": true, "group": true, "members": true, "interfaces": true, "node": true, "stub": true, "role": true}
		first := func(path string) string { return strings.SplitN(strings.SplitN(path, ".", 2)[0], "[", 2)[0] }
		m := netlabcaps.Manifest()
		if m.Core != nil {
			for _, sc := range m.Core.Scopes {
				switch sc.Name {
				case "link", "interface", "link_internal":
					for _, a := range sc.Attributes {
						linkAttrs[first(a.Path)] = true
					}
				}
			}
		}
		for _, mod := range m.Modules {
			linkAttrs[mod.Name] = true
		}
		for _, p := range m.Plugins {
			linkAttrs[first(p.Name)] = true
		}
	})
	return linkAttrs[first1(key)]
}

func first1(key string) string { return strings.SplitN(key, ".", 2)[0] }

func groupNodes(l *loader, groups []string) []*yaml.Node {
	var out []*yaml.Node
	for _, gn := range groups {
		out = append(out, l.groups[gn])
	}
	return out
}

// vlanRestriction reads evpn.vlans and vxlan.vlans from a node and its groups: when either is set, only the VLANs they list are
// carried over VXLAN (and judged by netlab's SVI check). nil means no restriction.
func vlanRestriction(sources []*yaml.Node) map[string]bool {
	var out map[string]bool
	for _, src := range sources {
		for _, key := range []string{"evpn.vlans", "vxlan.vlans"} {
			if v := getPath(src, key); v != nil {
				if out == nil {
					out = map[string]bool{}
				}
				for _, n := range stringList(v) {
					out[n] = true
				}
			}
		}
	}
	return out
}
