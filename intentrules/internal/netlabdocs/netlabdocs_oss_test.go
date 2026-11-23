package netlabdocs

import (
	"regexp"
	"strings"
	"testing"
)

// The OSS build must serve the open extensions, and rank them first: an author
// asks for a block by name and a page buried under caveats.md is no page.
func TestOSSCorpusCarriesTheOpenExtensions(t *testing.T) {
	for _, p := range []string{"module/sdwan.md", "module/services.md", "node/roles.md", "topology/otg.md"} {
		if _, ok := Read(p); !ok {
			t.Fatalf("%s missing from the OSS corpus", p)
		}
	}
	for term, want := range map[string]string{
		"otg":  "topology/otg.md",
		"ixia": "topology/otg.md",
	} {
		if h := Search(term); len(h) == 0 || h[0] != want {
			t.Fatalf("Search(%q) must rank %s first, got %v", term, want, h[:min(4, len(h))])
		}
	}
}

// The requirement: nothing of a private-scope netlab patch is reachable through
// Read, List or Search, and the OSS index does not name it.
func TestOSSCorpusServesNoPrivatePage(t *testing.T) {
	private := []string{
		"topology/forward.md", "topology/cloud.md", "topology/sase.md", "topology/wireless.md",
		"plugins/firewall.geneve.md", "devices/gobgp.md", "devices/gobgp-speaker.md",
	}
	for _, p := range private {
		if body, ok := Read(p); ok || body != "" {
			t.Fatalf("Read(%q) served a private page", p)
		}
		for _, e := range List() {
			if e.Path == p {
				t.Fatalf("the OSS index lists %s", p)
			}
		}
	}
	// POSITIVE CONTROL: the same calls must succeed on a page that is public, or
	// the loop above proves only that Read and List are empty.
	if _, ok := Read("topology/otg.md"); !ok || len(List()) < 90 {
		t.Fatalf("OSS corpus is empty or unreadable: %d entries", len(List()))
	}
	// Terms that exist only in the private pages must find nothing at all.
	for _, term := range []string{"gobgp", "geneve", "zscaler", "synthetic node"} {
		if h := Search(term); len(h) != 0 {
			t.Fatalf("Search(%q) found %v in the OSS corpus", term, h)
		}
	}
}

// No OSS page names Skyforge or the private integration, and its attributes are not documented.
func TestOSSKeepsUpstreamOnlyWording(t *testing.T) {
	for _, p := range []string{"caveats.md", "nodes.md", "links.md", "module/services.md", "topology/otg.md"} {
		body, ok := Read(p)
		if !ok {
			t.Fatalf("%s missing", p)
		}
		if strings.Contains(strings.ToLower(body), "skyforge") {
			t.Fatalf("%s still says Skyforge", p)
		}
	}
	for _, e := range List() {
		if body, _ := Read(e.Path); strings.Contains(strings.ToLower(body), "skyforge") {
			t.Fatalf("%s says Skyforge", e.Path)
		}
	}
	forward := regexp.MustCompile(`(?i)forward\.(synthetic|collect|connection|attach_to)`)
	for _, e := range List() {
		if body, _ := Read(e.Path); forward.MatchString(body) {
			t.Fatalf("%s carries private-integration wording", e.Path)
		}
	}
	// POSITIVE CONTROL: the upstream content of the same pages is still there.
	nodes, _ := Read("nodes.md")
	links, _ := Read("links.md")
	if !strings.Contains(nodes, "**lldp**") || !strings.Contains(links, "dictionary format of the link definition") {
		t.Fatal("the edited pages lost upstream content")
	}
}

var includeRe = regexp.MustCompile("(?m)^\\s*`{3,}\\{include\\}\\s+(\\S+)")

// Every `{include}` an OSS page names must exist in the OSS corpus (the private
// test uses the generator's own parser; the OSS module does not ship it).
func TestOSSEveryIncludeResolves(t *testing.T) {
	checked := 0
	for _, e := range List() {
		if !strings.HasSuffix(e.Path, ".md") {
			continue
		}
		body, _ := Read(e.Path)
		dir := e.Path[:max(strings.LastIndex(e.Path, "/"), 0)]
		for _, m := range includeRe.FindAllStringSubmatch(body, -1) {
			target := m[1]
			if dir != "" {
				target = dir + "/" + m[1]
			}
			if _, ok := Read(target); !ok {
				t.Errorf("%s includes %q but %s is not in the OSS corpus", e.Path, m[1], target)
			}
			checked++
		}
	}
	if checked < 12 {
		t.Fatalf("only %d includes found; the check is not looking at the pages that have them", checked)
	}
}
