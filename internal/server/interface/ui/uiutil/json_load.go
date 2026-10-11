package uiutil

import (
	"github.com/kipitix/growscada/internal/server/interface/ui/toast"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// JSONLoad describes the component-specific parts of a coalesced list load.
// Accept rejects a response for an obsolete selection and schedules a fresh
// load. Finished runs even on a failed or obsolete response.
type JSONLoad[T any] struct {
	URL      string
	Apply    func(app.Context, T)
	Reload   func(app.Context)
	Accept   func() bool
	Finished func()
}

// LoadJSON must receive the owning component's context. Transient controls
// can unmount while a request is in flight and lose their dispatched result.
func LoadJSON[T any](ctx app.Context, r *Reloader, load JSONLoad[T]) {
	if !r.Start() {
		return
	}
	ctx.Async(func() {
		result, problem := FetchJSON[T](load.URL)
		ctx.Dispatch(func(ctx app.Context) {
			rerun := r.Done()
			if load.Finished != nil {
				load.Finished()
			}
			switch {
			case load.Accept != nil && !load.Accept():
				rerun = true
			case problem != nil:
				ctx.NewActionWithValue(toast.ActionAdd, *problem)
			default:
				load.Apply(ctx, result)
			}
			if rerun {
				load.Reload(ctx)
			}
		})
	})
}
