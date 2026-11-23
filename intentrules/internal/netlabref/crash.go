package netlabref

// crash.go -- turn a Python traceback out of `netlab create` into a diagnostic a
// person (or a model) can act on.
//
// WHY. A netlab PLUGIN runs arbitrary Python against the author's values, and
// plugin code can reach a value of the wrong type before netlab's own validation
// has rejected it. The validate Job then prints a traceback, and the server used
// to keep exactly ONE line of it -- the literal "Traceback (most recent call
// last):" -- as the diagnostic and as the summary. Nobody reading that line, human
// or model, could tell which plugin had crashed or why, so the only reasonable
// move was to delete whatever the plugin was expanding until it compiled.
//
// This reads the traceback itself: the deepest frame inside netlab's plugin tree
// names the plugin, and the exception line is the reason. The advice that follows
// is GENERIC and derived from netlab: the plugin's declared attribute types, from
// the manifest. No plugin is special-cased here.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
)

// Crash is a Python exception raised while netlab ran.
type Crash struct {
	// Plugin is the netlab plugin whose code raised (or called into the code that
	// raised), as an author writes it in `plugin:`. Empty when no frame is inside
	// a plugin.
	Plugin string
	// File, Line and Func are the deepest frame inside netlab's tree.
	File string
	Line int
	Func string
	// ExcType and ExcMsg are the exception line, split at the first ": ".
	ExcType string
	ExcMsg  string
}

var (
	reTraceHeader = regexp.MustCompile(`Traceback \(most recent call last\)`)
	reTraceFrame  = regexp.MustCompile(`^File "([^"]+)", line (\d+), in (\S+)`)
	// An exception line: dotted class name ending in Error/Exception/Exit/Warning,
	// then an optional message. Anchored to the line start so a source line such as
	// `raise ValueError(...)` (which TrimSpace'd frames would otherwise match) is
	// not mistaken for it.
	reTraceExc = regexp.MustCompile(`^((?:[A-Za-z_][\w]*\.)*[A-Z]\w*(?:Error|Exception|Exit|Warning))(?::\s*(.*))?$`)
	// netsim/extra/<plugin>[/<sub>]/<file>.py -- the plugin tree.
	rePluginFrame = regexp.MustCompile(`netsim/extra/([^"]+)/[^/]+\.py$`)
	// The kubelet prefixes every Job log line with an RFC3339(Nano) timestamp when the server asks for timestamps=true.
	reLogStamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})(?:\s+|$)`)
)

// StripLogPrefix removes the kubelet's leading RFC3339 timestamp (and any CR/space around the line) so a log line
// can be matched by its text. A line without that prefix comes back trimmed and otherwise unchanged.
func StripLogPrefix(line string) string {
	line = strings.TrimSpace(line)
	return strings.TrimSpace(reLogStamp.ReplaceAllString(line, ""))
}

// ParseCrash finds the LAST traceback in lines and decodes it. The second return
// is false when there is no complete traceback (a header and an exception line).
// Lines may be indented, already trimmed, or carry the kubelet's leading RFC3339
// timestamp (the live path: timestamps=true): frames are recognised by their text,
// not their indentation or prefix.
func ParseCrash(lines []string) (Crash, bool) {
	start := -1
	for i, l := range lines {
		if reTraceHeader.MatchString(StripLogPrefix(l)) {
			start = i
		}
	}
	if start < 0 {
		return Crash{}, false
	}
	var c Crash
	var pluginDir string
	haveExc := false
	for _, raw := range lines[start+1:] {
		l := StripLogPrefix(raw)
		if l == "" {
			continue
		}
		if m := reTraceFrame.FindStringSubmatch(l); m != nil {
			if strings.Contains(m[1], "/netsim/") || strings.HasPrefix(m[1], "netsim/") {
				c.File = m[1]
				c.Line, _ = strconv.Atoi(m[2])
				c.Func = m[3]
			}
			if pm := rePluginFrame.FindStringSubmatch(m[1]); pm != nil {
				pluginDir = pm[1]
			}
			continue
		}
		if m := reTraceExc.FindStringSubmatch(l); m != nil {
			c.ExcType, c.ExcMsg = m[1], strings.TrimSpace(m[2])
			haveExc = true
			break
		}
		// anything else is source text, a caret marker, or a chained-exception note
	}
	if !haveExc {
		return Crash{}, false
	}
	c.Plugin = pluginName(pluginDir)
	return c, true
}

// pluginName maps netsim/extra's mixed one- and two-level layout back to the
// name an author writes: `firewall/policy` and the literal `firewall.policy`
// directory are both `firewall.policy`. The manifest decides, so a directory
// that is not a plugin is not named as one.
func pluginName(dir string) string {
	if dir == "" {
		return ""
	}
	dotted := strings.ReplaceAll(dir, "/", ".")
	if netlabcaps.HasPlugin(dotted) {
		return dotted
	}
	if first := strings.SplitN(dir, "/", 2)[0]; netlabcaps.HasPlugin(first) {
		return first
	}
	return ""
}

// ShortExc is the exception without its module path: BoxTypeError, not
// box.exceptions.BoxTypeError.
func (c Crash) ShortExc() string {
	if i := strings.LastIndex(c.ExcType, "."); i >= 0 {
		return c.ExcType[i+1:]
	}
	return c.ExcType
}

// excText is the exception as one sentence fragment: the short class name, then
// the message without its own trailing period (the caller adds punctuation).
func (c Crash) excText() string {
	exc := c.ShortExc()
	if m := strings.TrimRight(strings.TrimSpace(c.ExcMsg), ". "); m != "" {
		exc += ": " + m
	}
	return exc
}

func (c Crash) where() string {
	if c.File == "" {
		return ""
	}
	f := c.File
	if i := strings.Index(f, "netsim/"); i >= 0 {
		f = f[i:]
	}
	return fmt.Sprintf(" (%s line %d, in %s)", f, c.Line, c.Func)
}

// PluginCrashMessage is the diagnostic for a crash inside a plugin. It states the
// facts (which plugin, where, what Python said) and then the plugin's DECLARED
// attribute types, which are the thing the author compares their block against.
func PluginCrashMessage(c Crash) string {
	var b strings.Builder
	fmt.Fprintf(&b, "the plugin %q crashed while expanding its configuration%s: %s.", c.Plugin, c.where(), c.excText())
	s := SchemaFor(c.Plugin)
	if s.Known && len(s.Attributes) > 0 {
		fmt.Fprintf(&b, " Its attribute types are: %s.", Describe(s.Attributes))
		b.WriteString(" A crash here usually means a value under the plugin's block has a different type or shape than the plugin expects: compare each value you wrote with those declared types.")
	} else {
		b.WriteString(" netlab declares no attribute types for this plugin, so compare what you wrote with the plugin's documentation.")
	}
	if s.Doc != "" {
		fmt.Fprintf(&b, " Read the plugin's documentation: netlab_docs path=%s.", s.Doc)
	}
	b.WriteString(" Do not delete or simplify the part of the topology the plugin was expanding just to make it compile: that changes what was asked for; if the requested structure cannot be written with these types, say so.")
	return b.String()
}

// GenericCrashMessage is the diagnostic for a traceback that is not in a plugin
// (or in a plugin the manifest does not know). Still better than the header line.
func GenericCrashMessage(c Crash) string {
	return fmt.Sprintf("netlab raised a Python exception while compiling the topology%s: %s. "+
		"This is a crash, not a validation message; look for the value in the topology that this code reads.", c.where(), c.excText())
}

// crashPhrase is the text Classify and the server key on to recognise a
// plugin-crash message that has already been rendered into a diagnostic.
const crashPhrase = "crashed while expanding its configuration"

// IsPluginCrashText reports whether text carries a rendered plugin-crash message.
func IsPluginCrashText(text string) bool { return strings.Contains(text, crashPhrase) }
