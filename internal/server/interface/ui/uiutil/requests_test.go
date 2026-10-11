package uiutil

import (
	"context"
	"errors"
	"testing"
)

func TestLoad_ObsoleteFailureIsSuppressedAndLoadsCurrentSelection(t *testing.T) {
	var r Reloader
	var pending []func()
	reports, finished, idle, applied := 0, 0, 0, 0
	selection := "old"
	tasks := Tasks{Async: func(f func()) { pending = append(pending, f) }, Report: func(error) { reports++ }}
	var reload func()
	reload = func() {
		requested := selection
		Load(tasks, &r, LoadRequest[int]{
			Call: func(context.Context) (int, error) {
				if requested == "old" {
					return 0, errors.New("deleted scene")
				}
				return 42, nil
			},
			Accept: func() bool { return selection == requested },
			Apply:  func(v int) { applied = v }, Reload: reload,
			Finished: func() { finished++ }, Idle: func() { idle++ },
		})
	}
	reload()
	selection = "new"
	reload()
	reload()
	if len(pending) != 1 {
		t.Fatal("overlapping loads were not coalesced")
	}
	pending[0]()
	if len(pending) != 2 || reports != 0 || applied != 0 || idle != 0 {
		t.Fatalf("obsolete response escaped: queued=%d reports=%d applied=%d idle=%d", len(pending), reports, applied, idle)
	}
	pending[1]()
	if applied != 42 || finished != 2 || idle != 1 || reports != 0 {
		t.Fatalf("current selection not loaded: value=%d finished=%d idle=%d reports=%d", applied, finished, idle, reports)
	}
}

func TestRequest_DispatchesFailureOnOwnerAndPassesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var dispatched func()
	reported, failed := false, false
	Request(Tasks{Context: ctx, Dispatch: func(f func()) { dispatched = f }, Report: func(err error) {
		reported = errors.Is(err, context.Canceled)
	}}, func(ctx context.Context) (int, error) { return 0, ctx.Err() },
		func(int) { t.Fatal("canceled request succeeded") }, func() { failed = true })
	if reported || failed || dispatched == nil {
		t.Fatal("request mutated UI before dispatch")
	}
	dispatched()
	if !reported || !failed {
		t.Fatal("dispatched failure was lost")
	}
}
