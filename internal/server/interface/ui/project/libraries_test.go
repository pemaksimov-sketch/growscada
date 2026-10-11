package project

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
)

func TestAnalyzeLibraryPinImpact(t *testing.T) {
	libraryID := uuid.New()
	typeID := uuid.New()
	currentSnapshot := json.RawMessage(`{"widget_types":[{"id":"` + typeID.String() + `","name":"Gauge","input_ports":[{"name":"value"},{"name":"quality"}]}]}`)
	currentType := apiv0.WidgetTypeResponse{ID: typeID, Name: "Gauge", InputPorts: []apiv0.InputPort{{Name: "value"}, {Name: "quality"}}}
	bindings := []apiv0.PortBinding{{PortName: "value"}, {PortName: "quality"}}
	widgets := []sceneWidgets{{scene: apiv0.SceneResponse{Name: "Overview"}, items: []apiv0.WidgetResponse{{Name: "Pressure", TypeID: typeID, PortBindings: bindings}}}}
	pins := []apiv0.ProjectLibraryPinResponse{{LibraryID: libraryID, Release: apiv0.LibraryReleaseResponse{Content: currentSnapshot}}}

	t.Run("removed type is reported", func(t *testing.T) {
		impacts := analyzeLibraryPinImpact([]apiv0.WidgetTypeResponse{currentType}, pins, libraryID, &apiv0.LibraryReleaseResponse{Content: json.RawMessage(`{"widget_types":[]}`)}, widgets)
		if len(impacts) != 1 || impacts[0].issue != "WidgetType «Gauge» станет недоступен" {
			t.Fatalf("impacts = %#v", impacts)
		}
	})
	t.Run("removed bound port is reported", func(t *testing.T) {
		release := apiv0.LibraryReleaseResponse{Content: json.RawMessage(`{"widget_types":[{"id":"` + typeID.String() + `","name":"Gauge","input_ports":[{"name":"value"}]}]}`)}
		impacts := analyzeLibraryPinImpact([]apiv0.WidgetTypeResponse{currentType}, pins, libraryID, &release, widgets)
		if len(impacts) != 1 || impacts[0].issue != "потеряет входной порт «quality»" {
			t.Fatalf("impacts = %#v", impacts)
		}
	})
	t.Run("unrelated library change has no impact", func(t *testing.T) {
		impacts := analyzeLibraryPinImpact([]apiv0.WidgetTypeResponse{currentType}, pins, uuid.New(), nil, widgets)
		if len(impacts) != 0 {
			t.Fatalf("impacts = %#v", impacts)
		}
	})
}
