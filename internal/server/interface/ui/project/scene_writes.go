package project

import (
	"context"

	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
)

// A Scene and its Widgets share one optimistic-lock version. Serialize writes
// from this editor, advancing only from successful responses; an external
// conflict still fails normally and cancels dependent local writes.
type sceneWrite func(version int, complete func(version int, ok bool))
type sceneWrites struct {
	version                      int
	pending                      []sceneWrite
	refreshScene, refreshWidgets bool
}

func (p *Project) writesFor(sceneID string) *sceneWrites {
	return p.sceneWrites[sceneID]
}

func sceneRequest[T any](p *Project, sceneID string, call func(context.Context, int) (T, error), version func(T) int, success func(T)) {
	if p.sceneWrites == nil {
		p.sceneWrites = make(map[string]*sceneWrites)
	}
	q := p.sceneWrites[sceneID]
	running := q != nil
	if q == nil {
		sc, found := p.findScene(sceneID)
		if !found {
			return
		}
		q = &sceneWrites{version: sc.Version}
		p.sceneWrites[sceneID] = q
	}
	q.pending = append(q.pending, func(v int, complete func(int, bool)) {
		uiutil.Request(p.tasks, func(ctx context.Context) (T, error) { return call(ctx, v) }, func(result T) {
			v = version(result)
			p.applyNewSceneVersion(sceneID, v)
			if success != nil {
				success(result)
			}
			complete(v, true)
		}, func() { complete(0, false) })
	})
	if !running {
		p.nextSceneWrite(sceneID, q)
	}
}

func (p *Project) nextSceneWrite(sceneID string, q *sceneWrites) {
	write := q.pending[0]
	q.pending = q.pending[1:]
	write(q.version, func(version int, ok bool) {
		if ok {
			q.version = version
			if len(q.pending) > 0 {
				p.nextSceneWrite(sceneID, q)
				return
			}
		}
		delete(p.sceneWrites, sceneID)
		if !ok || q.refreshScene {
			p.loadScene(p.compoCtx, sceneID)
		}
		if (!ok || q.refreshWidgets) && p.selectedSceneID == sceneID {
			p.loadWidgets(p.compoCtx)
		}
	})
}
