package netlabref

import (
	"strings"
	"testing"
	"time"
)

// fabric_dict_counts_job_output.txt is REAL output: a topology whose fabric plugin block gives superspines/spines/leafs as
// mappings, run through `netlab create` the way the validate Job runs it, in the netlab runtime image
// (python traceback plus the Job's own
// "ERROR: netlab create failed: ..." line).

func jobLines(t *testing.T, trim bool) []string {
	t.Helper()
	var out []string
	for _, l := range strings.Split(readTestdata(t, "fabric_dict_counts_job_output.txt"), "\n") {
		if trim {
			// internal/taskengine's capturingLogger.lines() trims every line, destroying indentation.
			l = strings.TrimSpace(l)
		}
		out = append(out, l)
	}
	return out
}

func TestParseCrashFindsThePluginInARealTraceback(t *testing.T) {
	for _, trim := range []bool{false, true} {
		c, ok := ParseCrash(jobLines(t, trim))
		if !ok {
			t.Fatalf("trim=%v: no crash found in a real traceback", trim)
		}
		if c.Plugin != "fabric" {
			t.Errorf("trim=%v: plugin = %q, want fabric", trim, c.Plugin)
		}
		if c.Func != "generate_fabric" || c.Line != 127 || !strings.HasSuffix(c.File, "netsim/extra/fabric/__init__.py") {
			t.Errorf("trim=%v: location = %s:%d in %s", trim, c.File, c.Line, c.Func)
		}
		if c.ShortExc() != "BoxTypeError" || !strings.HasPrefix(c.ExcMsg, "Box can only merge two boxes") {
			t.Errorf("trim=%v: exception = %q: %q", trim, c.ExcType, c.ExcMsg)
		}
	}
}

func TestPluginCrashMessageNamesThePluginAndItsDeclaredTypes(t *testing.T) {
	c, _ := ParseCrash(jobLines(t, true))
	msg := PluginCrashMessage(c)
	for _, must := range []string{
		`the plugin "fabric" crashed while expanding its configuration`,
		"BoxTypeError",
		"fabric.leafs (int, required)",
		"fabric.spines (int, required)",
		"netlab_docs path=plugins/fabric.md",
		"Do not delete or simplify",
	} {
		if !strings.Contains(msg, must) {
			t.Errorf("message lacks %q:\n%s", must, msg)
		}
	}
	// GENERIC: nothing in the message may be fabric-specific advice (no tier names, no "use integers for superspines").
	for _, banned := range []string{"superspine", "tier", "dict leaf"} {
		if strings.Contains(strings.ToLower(msg), banned) {
			t.Errorf("per-plugin gotcha text leaked into a generic message: %q in\n%s", banned, msg)
		}
	}
}

func TestAPluginCrashIsNeitherAGapNorAGuess(t *testing.T) {
	c, _ := ParseCrash(jobLines(t, true))
	h := Classify(PluginCrashMessage(c))
	if h.Class != ClassPluginCrash {
		t.Errorf("class = %q, want %q", h.Class, ClassPluginCrash)
	}
	if h.Hint == GapHintNotModelled {
		t.Error("a plugin crash must never be filed as a netlab gap on a guess")
	}
}

func TestTracebackOutsideAPluginGetsAGenericMessage(t *testing.T) {
	lines := []string{
		"Traceback (most recent call last):",
		`File "/opt/netlab/netsim/augment/nodes.py", line 88, in create_node_dict`,
		"x = 1 / 0",
		"ZeroDivisionError: division by zero",
		"ERROR: netlab create failed: division by zero",
	}
	c, ok := ParseCrash(lines)
	if !ok || c.Plugin != "" {
		t.Fatalf("want a crash with no plugin, got %+v ok=%v", c, ok)
	}
	msg := GenericCrashMessage(c)
	if !strings.Contains(msg, "ZeroDivisionError: division by zero") || !strings.Contains(msg, "netsim/augment/nodes.py line 88") {
		t.Errorf("generic message hides the cause: %s", msg)
	}
	if IsPluginCrashText(msg) {
		t.Error("a non-plugin crash was labelled a plugin crash")
	}
}

func TestNoTracebackMeansNoCrash(t *testing.T) {
	if _, ok := ParseCrash([]string{"ERROR: IncorrectAttr in links: Invalid link attribute 'eigpr' found in links[1]"}); ok {
		t.Error("netlab's own validation message was read as a crash")
	}
	if _, ok := ParseCrash([]string{"Traceback (most recent call last):", "File \"/x/netsim/extra/fabric/__init__.py\", line 1, in f"}); ok {
		t.Error("a traceback with no exception line was accepted")
	}
}

// The server never sees the Job's bytes: kube_jobs_api asks the kubelet for timestamps=true, so every line arrives as
// "<RFC3339Nano> <text>", and capturingLogger.lines() then TrimSpaces the whole line (the text's own indentation survives
// after the timestamp). ParseCrash used to anchor frames at "^File", so on this path it found no frame and no
// exception and the caller got the bare traceback header.
func kubeletLines(t *testing.T, file string) []string {
	t.Helper()
	var out []string
	for i, l := range strings.Split(readTestdata(t, file), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		ts := time.Date(2026, 11, 3, 23, 41, 12, 123456789+i, time.UTC).Format(time.RFC3339Nano)
		out = append(out, strings.TrimSpace(ts+" "+l))
	}
	return out
}

func TestParseCrashSurvivesTheKubeletTimestampPrefix(t *testing.T) {
	for _, file := range []string{"fabric_dict_counts_job_output.txt", "fabric_list_vs_dict_typeerror_job_output.txt"} {
		for name, lines := range map[string][]string{
			"prefixed":        kubeletLines(t, file),
			"prefixed+CRLF":   crlf(kubeletLines(t, file)),
			"unprefixed":      strings.Split(readTestdata(t, file), "\n"),
			"unprefixed+trim": trimmed(strings.Split(readTestdata(t, file), "\n")),
		} {
			c, ok := ParseCrash(lines)
			if !ok {
				t.Fatalf("%s/%s: no crash found", file, name)
			}
			if c.Plugin != "fabric" || !strings.HasSuffix(c.File, "netsim/extra/fabric/__init__.py") {
				t.Errorf("%s/%s: plugin=%q file=%q", file, name, c.Plugin, c.File)
			}
			if c.ExcType == "" || strings.Contains(c.ExcType, "Z") && strings.HasPrefix(c.ExcType, "20") {
				t.Errorf("%s/%s: exception type carries the prefix: %q", file, name, c.ExcType)
			}
		}
	}
}

func TestTypeErrorCrashNamesTheExceptionAndLocation(t *testing.T) {
	c, ok := ParseCrash(kubeletLines(t, "fabric_list_vs_dict_typeerror_job_output.txt"))
	if !ok {
		t.Fatal("no crash")
	}
	if c.ExcType != "TypeError" || c.ExcMsg != `can only concatenate list (not "dict") to list` {
		t.Errorf("exception = %q: %q", c.ExcType, c.ExcMsg)
	}
	if c.Func != "adjust_group_parameters" || c.Line != 91 {
		t.Errorf("location = %d in %s", c.Line, c.Func)
	}
}

func TestStripLogPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"2026-11-03T23:41:12.123456789Z   File \"x\", line 1, in f": "File \"x\", line 1, in f",
		"2026-11-03T23:41:12Z Traceback (most recent call last):":   "Traceback (most recent call last):",
		"2026-11-03T23:41:12.5+01:00 ERROR: boom":                   "ERROR: boom",
		"File \"x\", line 1, in f":                                  "File \"x\", line 1, in f",
		"2026-11-03T23:41:12Z":                                      "",
		"20261103 not a stamp":                                      "20261103 not a stamp",
	} {
		if got := StripLogPrefix(in); got != want {
			t.Errorf("StripLogPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func crlf(in []string) []string {
	out := make([]string, len(in))
	for i, l := range in {
		out[i] = l + "\r"
	}
	return out
}

func trimmed(in []string) []string {
	out := make([]string, len(in))
	for i, l := range in {
		out[i] = strings.TrimSpace(l)
	}
	return out
}
