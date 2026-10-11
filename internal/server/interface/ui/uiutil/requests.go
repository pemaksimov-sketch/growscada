package uiutil

import (
	"context"

	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Tasks separates scheduling, storage and notifications from UI state. Tests
// can run the same operations synchronously using the Memory API adapter.
type Tasks struct {
	Context  context.Context
	Async    func(func())
	Dispatch func(func())
	Remember func(string, any)
	Report   func(error)
}

func BrowserTasks(ctx app.Context) Tasks {
	return Tasks{Context: ctx, Async: ctx.Async,
		Dispatch: func(f func()) { ctx.Dispatch(func(app.Context) { f() }) },
		Remember: func(key string, value any) { ctx.LocalStorage().Set(key, value) },
		Report:   func(err error) { ctx.NewActionWithValue(toast.ActionAdd, toast.FromClientError(err)) },
	}
}

func (t Tasks) Store(key string, value any) {
	if t.Remember != nil {
		t.Remember(key, value)
	}
}

// Request dispatches success or failure exactly once through the owning
// component. It never mutates UI state on the background request goroutine.
func Request[T any](t Tasks, call func(context.Context) (T, error), success func(T), failure func()) {
	async, dispatch := t.Async, t.Dispatch
	if async == nil {
		async = func(f func()) { f() }
	}
	if dispatch == nil {
		dispatch = func(f func()) { f() }
	}
	ctx := t.Context
	if ctx == nil {
		ctx = context.Background()
	}
	async(func() {
		result, err := call(ctx)
		dispatch(func() {
			if err != nil {
				if t.Report != nil {
					t.Report(err)
				}
				if failure != nil {
					failure()
				}
				return
			}
			if success != nil {
				success(result)
			}
		})
	})
}

type LoadRequest[T any] struct {
	Call     func(context.Context) (T, error)
	Apply    func(T)
	Reload   func()
	Accept   func() bool
	Finished func()
	Idle     func()
}

type loadResult[T any] struct {
	value T
	err   error
}

func Load[T any](tasks Tasks, r *Reloader, load LoadRequest[T]) {
	if !r.Start() {
		return
	}
	Request(tasks, func(ctx context.Context) (loadResult[T], error) {
		value, err := load.Call(ctx)
		return loadResult[T]{value: value, err: err}, nil
	}, func(result loadResult[T]) {
		rerun := r.Done()
		if load.Finished != nil {
			load.Finished()
		}
		if load.Accept != nil && !load.Accept() {
			rerun = true
		} else if result.err != nil {
			if tasks.Report != nil {
				tasks.Report(result.err)
			}
		} else if load.Apply != nil {
			load.Apply(result.value)
		}
		if rerun {
			load.Reload()
		} else if load.Idle != nil {
			load.Idle()
		}
	}, nil)
}
