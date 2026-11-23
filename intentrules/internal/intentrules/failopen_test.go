package intentrules

import (
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestGuardReturnsTheClassOfAPanicAndNothingElse(t *testing.T) {
	if f := Guard(func() {}); f != "" {
		t.Errorf("a clean run must return no fault, got %q", f)
	}
	f := Guard(func() { panic("document text that must not be logged") })
	if f != "panic:string" || strings.Contains(f, "document") {
		t.Errorf("fault = %q; want the type only", f)
	}
	f = Guard(func() { var m map[string]int; m["x"] = 1 })
	if !strings.HasPrefix(f, "panic:") {
		t.Errorf("a runtime panic must be recovered: %q", f)
	}
}

// Positive control for the fail-open tests in the packages that call this one: with the fault injected and no Guard, evaluating the
// document panics, so those tests passing means the recover is what saved them.
func TestInjectedFaultsReallyBreakEvaluationAndLoading(t *testing.T) {
	e := engine(t)
	doc, _ := os.ReadFile("testdata/evpn-clean.yml")
	restore := InjectFaultsForTest(true, errors.New("injected load failure"))
	defer restore()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("with the fault injected, an unguarded evaluation must panic")
			}
		}()
		e.InspectYAML(string(doc), Options{})
	}()
	if _, err := Default(); err == nil {
		t.Error("with a load fault injected, Default must fail")
	}
	if f := Guard(func() { e.DesignerIssues(string(doc)) }); f == "" {
		t.Error("Guard must report the injected panic")
	}
	restore()
	if _, err := Default(); err != nil {
		t.Errorf("after restore the model must load: %v", err)
	}
	if got := e.InspectYAML(string(doc), Options{}); len(got.Evaluated) == 0 {
		t.Error("after restore evaluation must run")
	}
}

// The binary carries the attribution Apache-2.0 asks for beside the schema it embeds.
func TestTheEmbeddedSchemaCarriesItsNoticeAndLicence(t *testing.T) {
	notice, licence, err := ThirdPartyNotices()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(notice, "Apache License 2.0") || !strings.Contains(notice, "c19896447c0055f577e8dc60718398821952c68b37385596d6758907183a98fe") {
		t.Errorf("NOTICE does not name the licence and the schema's digest")
	}
	if !strings.Contains(licence, "Apache License") || !strings.Contains(licence, "Version 2.0") {
		t.Errorf("the licence text is not Apache-2.0")
	}
}

// The plugin-owner table is built lazily; concurrent first use must not race (run with -race to prove it).
func TestPluginOwnersAreSafeForConcurrentFirstUse(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if pluginBlockOwners()["fabric"] != "fabric" {
				t.Error("fabric's block must be owned by the fabric plugin")
			}
		}()
	}
	wg.Wait()
}

// The seam is zero-cost when off (no lock, no allocation) and restore puts back exactly the state it replaced, including nested use.
func TestFaultSeamIsOffByDefaultAllocationFreeAndRestoresNesting(t *testing.T) {
	if err := injectedLoadError(); err != nil {
		t.Fatalf("the seam must be off by default, load error %v", err)
	}
	maybePanicForTest() // must not panic when off
	if n := testing.AllocsPerRun(100, func() { maybePanicForTest(); _ = injectedLoadError() }); n != 0 {
		t.Errorf("the seam allocates %v per call when off", n)
	}
	outerErr := errors.New("outer")
	outer := InjectFaultsForTest(false, outerErr)
	inner := InjectFaultsForTest(true, nil)
	if injectedLoadError() != nil || !faults.panicInEval.Load() {
		t.Error("the inner injection must replace the outer one")
	}
	inner()
	if injectedLoadError() != outerErr || faults.panicInEval.Load() {
		t.Error("restoring the inner injection must give back the outer one")
	}
	outer()
	if injectedLoadError() != nil || faults.panicInEval.Load() {
		t.Error("restoring the outer injection must switch the seam off")
	}
}
