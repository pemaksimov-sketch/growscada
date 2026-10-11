package application

import (
	"context"
	"errors"
	"fmt"
	"github.com/kipitix/growscada/internal/server/domain/event"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/scene"
)

// WidgetReconciliationPolicy reconciles each Scene in its own transaction.
// Repeating a successful delivery changes no Scene and emits no new events.
// Always load the current type: an old retry must not restore obsolete ports.
type WidgetReconciliationPolicy struct {
	repository      library.WidgetTypeRepository
	sceneRepository scene.SceneRepository
	scopes          func(context.Context) ([]WidgetBindingScope, error)
}

func NewWidgetReconciliationPolicy(types library.WidgetTypeRepository, scenes scene.SceneRepository) *WidgetReconciliationPolicy {
	return &WidgetReconciliationPolicy{repository: types, sceneRepository: scenes}
}

// Until Library Release pinning (task 33), a shared type update reconciles
// every project's scenes through that project's repositories. No unscoped
// repository or request context is used by this background subscriber.
func NewProjectWidgetReconciliationPolicy(types library.WidgetTypeRepository, scopes func(context.Context) ([]WidgetBindingScope, error)) *WidgetReconciliationPolicy {
	return &WidgetReconciliationPolicy{repository: types, scopes: scopes}
}

func (s *WidgetReconciliationPolicy) Handle(ctx context.Context, e event.Event) error {
	if s.scopes != nil {
		scopes, err := s.scopes(ctx)
		if err != nil {
			return err
		}
		for _, scope := range scopes {
			if err := NewWidgetReconciliationPolicy(s.repository, scope.Scenes).Handle(ctx, e); err != nil {
				return err
			}
		}
		return nil
	}
	changed, ok := e.(library.WidgetTypeUpdatedEvent)
	if !ok || e.Type() != event.EventTypeWidgetTypeUpdated {
		return fmt.Errorf("reconciliation: unexpected event %s", e.Type())
	}
	wt, err := s.repository.FindByID(ctx, changed.WidgetTypeID())
	if errors.Is(err, library.ErrWidgetTypeNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.removeOrphanedPortBindings(ctx, wt)
}

// removeOrphanedPortBindings removes, from every Widget of WidgetType wt,
// the PortBindings of ports wt no longer declares: each Scene holding such
// Widgets reconciles them with wt and is saved, with the events it recorded,
// if anything changed. Each Scene is saved in its own transaction, one by
// one, from a durable widget_type_updated subscription.
func (s *WidgetReconciliationPolicy) removeOrphanedPortBindings(ctx context.Context, wt library.WidgetType) error {
	scenes, err := s.sceneRepository.FindByWidgetTypeID(ctx, wt.ID())
	if err != nil {
		return fmt.Errorf("cannot list scenes with widgets of type %s: %w", wt.ID(), err)
	}

	for _, sc := range scenes {
		if err := s.reconcileScene(ctx, sc, wt); err != nil {
			return fmt.Errorf("cannot save scene %s after port binding cleanup: %w", sc.ID(), err)
		}
	}
	return nil
}

// reconcileScene reconciles the Scene sc with wt and saves it if anything
// changed. The change does not rest on what a client saw, so a Scene changed
// since it was read is reread and reconciled again rather than reported as a
// conflict (retryOnRace); a Scene deleted meanwhile has nothing left to clean.
func (s *WidgetReconciliationPolicy) reconcileScene(ctx context.Context, sc scene.Scene, wt library.WidgetType) error {
	first := true
	err := retryOnRace(scene.ErrSceneConflict, func() error {
		if !first {
			var err error
			if sc, err = s.sceneRepository.FindByID(ctx, sc.ID()); err != nil {
				return fmt.Errorf("cannot reread scene: %w", err)
			}
		}
		first = false
		reconciled, changed := sc.ReconcileWith(wt)
		if !changed {
			return nil
		}
		_, err := s.sceneRepository.Save(ctx, reconciled)
		return err
	})
	if errors.Is(err, scene.ErrSceneNotFound) {
		return nil
	}
	return err
}
