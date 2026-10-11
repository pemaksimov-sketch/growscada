package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/application"
	"github.com/kipitix/growscada/internal/server/application/appdto"
)

func createBoundTag(t *testing.T) uuid.UUID {
	t.Helper()
	tag, err := newService().CreateTag(context.Background(), appdto.CreateTagInput{Name: "binding-" + uuid.NewString(), Type: "integer", Value: "1", Quality: "good"})
	if err != nil {
		t.Fatal(err)
	}
	return tag.ID
}

func TestPortBindingTypeValidation(t *testing.T) {
	for _, tc := range []struct {
		hint, tagType, value string
		invalid              bool
	}{
		{"integer", "integer", "1", false}, {"boolean", "boolean", "true", false}, {"string", "string", "x", false},
		{"", "string", "x", false}, {"", "integer", "1", false}, {"", "boolean", "false", false},
		{"integer", "string", "x", true}, {"integer", "boolean", "true", true}, {"boolean", "integer", "1", true},
	} {
		t.Run(tc.hint+"_"+tc.tagType, func(t *testing.T) {
			cleanScenes(t)
			ctx := context.Background()
			tag, err := newService().CreateTag(ctx, appdto.CreateTagInput{Name: "binding-" + uuid.NewString(), Type: tc.tagType, Value: tc.value, Quality: "good"})
			if err != nil {
				t.Fatal(err)
			}
			wtInput := testCreateWidgetTypeInput
			wtInput.InputPorts = []appdto.InputPort{{Name: "value", TypeHint: tc.hint}}
			wt, err := newWidgetTypeService().CreateWidgetType(ctx, wtInput)
			if err != nil {
				t.Fatal(err)
			}
			svc := newSceneService()
			sc := mustCreateScene(t, svc)
			input := testCreateWidgetInput
			input.TypeID = wt.ID
			input.PortBindings = []appdto.PortBinding{{PortName: "value", TagID: tag.ID}}
			_, err = svc.CreateWidget(ctx, sc.ID, sc.Version, input)
			if errors.Is(err, application.ErrInvalidInput) != tc.invalid || (!tc.invalid && err != nil) {
				t.Fatalf("CreateWidget: %v", err)
			}
			if tc.invalid {
				input.PortBindings = nil
				created, err := svc.CreateWidget(ctx, sc.ID, sc.Version, input)
				if err != nil {
					t.Fatal(err)
				}
				input.PortBindings = []appdto.PortBinding{{PortName: "value", TagID: tag.ID}}
				if _, err := svc.UpdateWidget(ctx, sc.ID, created.ID, created.SceneVersion, input); !errors.Is(err, application.ErrInvalidInput) {
					t.Fatalf("UpdateWidget: %v", err)
				}
			}
		})
	}
}

func TestPortBindingMissingTagAndTypeHintChange(t *testing.T) {
	cleanScenes(t)
	ctx := context.Background()
	wtSvc := newWidgetTypeService()
	wtInput := testCreateWidgetTypeInput
	wtInput.InputPorts = []appdto.InputPort{{Name: "value", TypeHint: ""}}
	wt, err := wtSvc.CreateWidgetType(ctx, wtInput)
	if err != nil {
		t.Fatal(err)
	}
	svc := newSceneService()
	sc := mustCreateScene(t, svc)
	input := testCreateWidgetInput
	input.TypeID = wt.ID
	input.PortBindings = []appdto.PortBinding{{PortName: "value", TagID: uuid.New()}}
	if _, err := svc.CreateWidget(ctx, sc.ID, sc.Version, input); !errors.Is(err, application.ErrInvalidInput) {
		t.Fatalf("missing tag: %v", err)
	}
	tag, err := newService().CreateTag(ctx, appdto.CreateTagInput{Name: "binding-" + uuid.NewString(), Type: "string", Value: "x", Quality: "good"})
	if err != nil {
		t.Fatal(err)
	}
	input.PortBindings[0].TagID = tag.ID
	widget, err := svc.CreateWidget(ctx, sc.ID, sc.Version, input)
	if err != nil {
		t.Fatal(err)
	}
	wtInput.InputPorts[0].TypeHint = "integer"
	if _, err := wtSvc.UpdateWidgetType(ctx, wt.ID, wt.Version, wtInput); !errors.Is(err, application.ErrInvalidInput) {
		t.Fatalf("narrowed hint: %v", err)
	}
	unchanged, err := wtSvc.FindWidgetTypeByID(ctx, wt.ID)
	if err != nil || unchanged.Version != wt.Version || unchanged.InputPorts[0].TypeHint != "" {
		t.Fatalf("rejected update changed definition: %+v %v", unchanged, err)
	}
	saved, err := svc.FindWidgetByID(ctx, sc.ID, widget.ID)
	if err != nil || saved.SceneVersion != widget.SceneVersion || len(saved.PortBindings) != 1 {
		t.Fatalf("rejected update changed widget: %+v %v", saved, err)
	}
	wtInput.InputPorts[0].TypeHint = "string"
	updated, err := wtSvc.UpdateWidgetType(ctx, wt.ID, wt.Version, wtInput)
	if err != nil {
		t.Fatalf("compatible hint: %v", err)
	}
	wtInput.InputPorts[0].TypeHint = ""
	if _, err := wtSvc.UpdateWidgetType(ctx, wt.ID, updated.Version, wtInput); err != nil {
		t.Fatalf("widened hint: %v", err)
	}
}
