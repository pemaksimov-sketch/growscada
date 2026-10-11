package growctl_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/growctl"
)

func TestParseWidgetTypeManifest_RequiresLibraryAndValidPorts(t *testing.T) {
	libID := uuid.NewString()
	doc := `apiVersion: growscada/v0.3
kind: WidgetType
metadata:
  name: Gauge
  library: ` + libID + `
spec:
  htmlTemplate: "<div></div>"
  script: ""
  scriptLanguage: javascript
  defaultWidth: 120
  defaultHeight: 60
  inputPorts:
    - name: value
      description: Process value
      typeHint: integer
`
	items, err := growctl.ParseWidgetTypeManifests([]byte(doc))
	if err != nil || len(items) != 1 {
		t.Fatalf("parse: %#v, %v", items, err)
	}
	if items[0].LibraryID.String() != libID || items[0].Name != "Gauge" || len(items[0].InputPorts) != 1 || items[0].InputPorts[0].TypeHint != "integer" {
		t.Fatalf("unexpected manifest: %#v", items[0])
	}
	for name, input := range map[string]string{
		"missing library": strings.Replace(doc, "  library: "+libID+"\n", "", 1),
		"duplicate ports": strings.Replace(doc, "    - name: value\n      description: Process value\n      typeHint: integer\n", "    - name: value\n      description: Process value\n      typeHint: integer\n    - name: value\n", 1),
		"unknown field":   strings.Replace(doc, "  name: Gauge\n", "  name: Gauge\n  unrecognized: true\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := growctl.ParseWidgetTypeManifests([]byte(input)); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
