package apiclient_test

import (
	"context"
	"errors"
	"testing"

	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/apiclient"
)

func TestMemory_SnapshotsAreIsolatedAndCanceledWriteDoesNotCommit(t *testing.T) {
	ctx := context.Background()
	m := apiclient.NewMemory()
	input := apiv0.CreateWidgetTypeRequest{Name: "type", InputPorts: []apiv0.InputPort{{Name: "n"}}}
	created, err := m.CreateWidgetType(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.InputPorts[0].Name = "changed after creation"
	first, _ := m.GetWidgetType(ctx, created.ID)
	if first.InputPorts[0].Name != "n" {
		t.Fatal("request aliases stored state")
	}
	first.InputPorts[0].Name = "changed snapshot"
	list, _ := m.ListWidgetTypes(ctx)
	if list[0].InputPorts[0].Name != "n" {
		t.Fatal("response aliases stored state")
	}
	list[0].InputPorts[0].Name = "changed list"
	second, _ := m.GetWidgetType(ctx, created.ID)
	if second.InputPorts[0].Name != "n" {
		t.Fatal("list aliases stored state")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = m.DeleteWidgetType(canceled, created.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if _, err = m.GetWidgetType(ctx, created.ID); err != nil {
		t.Fatalf("canceled delete committed: %v", err)
	}
}
