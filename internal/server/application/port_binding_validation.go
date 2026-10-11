package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/scene"
	"github.com/kipitix/growscada/internal/server/domain/tag"
)

// Only the listed ports are checked. Widget writes supply all ports after the
// Scene checks their names; WidgetType edits supply only changed type hints.
func validatePortBindingTypes(ctx context.Context, repo tag.TagRepository, ports []library.InputPort, bindings []scene.PortBinding) error {
	hints := make(map[library.InputPortName]library.PortTypeHint, len(ports))
	for _, port := range ports {
		hints[port.Name()] = port.TypeHint()
	}
	for _, binding := range bindings {
		hint, check := hints[binding.PortName()]
		if !check {
			continue
		}
		t, err := repo.FindByID(ctx, binding.TagID())
		if errors.Is(err, tag.ErrTagNotFound) {
			return invalidInput(fmt.Errorf("port_bindings: port %q references missing tag %s", binding.PortName(), binding.TagID()))
		}
		if err != nil {
			return fmt.Errorf("cannot find tag for port %q: %w", binding.PortName(), err)
		}
		if !hint.Accepts(t.Type()) {
			return invalidInput(fmt.Errorf("port_bindings: port %q accepts %s, tag %s has type %s", binding.PortName(), hint, binding.TagID(), t.Type()))
		}
	}
	return nil
}

func (s widgetTypeServiceImpl) validateChangedTypeHints(ctx context.Context, old, updated library.WidgetType) error {
	previous := make(map[library.InputPortName]library.PortTypeHint)
	for _, port := range old.InputPorts() {
		previous[port.Name()] = port.TypeHint()
	}
	var changed []library.InputPort
	for _, port := range updated.InputPorts() {
		if hint, exists := previous[port.Name()]; exists && hint != port.TypeHint() && !port.TypeHint().IsAny() {
			changed = append(changed, port)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	scopes := []WidgetBindingScope{{Scenes: s.sceneRepository, Tags: s.tagRepository}}
	if s.bindingScopes != nil {
		var err error
		scopes, err = s.bindingScopes(ctx)
		if err != nil {
			return err
		}
	}
	for _, scope := range scopes {
		if err := validateChangedBindingsInScope(ctx, scope, updated, changed); err != nil {
			return err
		}
	}
	return nil
}

func validateChangedBindingsInScope(ctx context.Context, scope WidgetBindingScope, updated library.WidgetType, changed []library.InputPort) error {
	scenes, err := scope.Scenes.FindByWidgetTypeID(ctx, updated.ID())
	if err != nil {
		return fmt.Errorf("cannot inspect existing port bindings: %w", err)
	}
	for _, sc := range scenes {
		for _, widget := range sc.Widgets() {
			if widget.TypeID() != updated.ID() {
				continue
			}
			if err := validatePortBindingTypes(ctx, scope.Tags, changed, widget.PortBindings()); err != nil {
				return fmt.Errorf("scene %s widget %s: %w", sc.ID(), widget.ID(), err)
			}
		}
	}
	return nil
}
