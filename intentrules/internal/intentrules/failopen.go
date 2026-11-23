package intentrules

import (
	"fmt"
	"sync/atomic"
)

// The layer is advisory: a defect in it must never change what validate or the designer do without it. Callers therefore run it under
// Guard and treat a load error from Default as "layer absent", logging both. Neither carries user content: a panic is logged by its
// type only (a panic message may quote the document), a load error by the embedded model's own message.

// Guard runs fn and returns "" when it completed, or the class of the panic when it did not ("panic:runtime.boundsError").
func Guard(fn func()) (fault string) {
	defer func() {
		if r := recover(); r != nil {
			fault = fmt.Sprintf("panic:%T", r)
		}
	}()
	fn()
	return ""
}

// faults are seams for tests in other packages (which cannot reach unexported state): they force the two failure modes the callers
// must survive. They are off unless a test sets them. The production cost of the seam is one atomic load per evaluation and one per
// model load (no lock, no allocation): the exported setter has to exist in the shipped binary because the tests that use it live in
// other packages, and a build tag would hide it from `encore test`.
var faults struct {
	panicInEval atomic.Bool
	loadErr     atomic.Pointer[error]
}

// InjectFaultsForTest makes every evaluation panic and/or Default return loadErr, until the returned function is called.
func InjectFaultsForTest(panicInEval bool, loadErr error) (restore func()) {
	prevP, prevE := faults.panicInEval.Load(), faults.loadErr.Load()
	faults.panicInEval.Store(panicInEval)
	if loadErr == nil {
		faults.loadErr.Store(nil)
	} else {
		faults.loadErr.Store(&loadErr)
	}
	return func() {
		faults.panicInEval.Store(prevP)
		faults.loadErr.Store(prevE)
	}
}

func injectedLoadError() error {
	if p := faults.loadErr.Load(); p != nil {
		return *p
	}
	return nil
}

func maybePanicForTest() {
	if faults.panicInEval.Load() {
		panic("injected fault")
	}
}
