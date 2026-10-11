package growctl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	manifestv0 "github.com/kipitix/growscada/contract/manifest/v0"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"gopkg.in/yaml.v3"
)

// WidgetTypeManifest declares one mutable WidgetType in a Library Draft.
type WidgetTypeManifest struct {
	LibraryID      uuid.UUID
	Name           string
	HtmlTemplate   string
	Script         string
	ScriptLanguage string
	DefaultWidth   int
	DefaultHeight  int
	InputPorts     []manifestv0.WidgetPort
}

// ReadWidgetTypeManifests reads WidgetType documents from YAML files or stdin.
func ReadWidgetTypeManifests(sources []string, stdin io.Reader) ([]WidgetTypeManifest, error) {
	if len(sources) == 0 {
		return nil, errNoManifest
	}
	var all []WidgetTypeManifest
	for _, source := range sources {
		files, err := expandSource(source)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			var data []byte
			if file == "-" {
				data, err = io.ReadAll(stdin)
			} else {
				data, err = os.ReadFile(file)
			}
			if err != nil {
				return nil, fmt.Errorf("cannot read %s: %w", file, err)
			}
			items, err := ParseWidgetTypeManifests(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			all = append(all, items...)
		}
	}
	seen := make(map[string]bool, len(all))
	for _, item := range all {
		key := item.LibraryID.String() + "/" + item.Name
		if seen[key] {
			return nil, fmt.Errorf("widgettype/%s is declared more than once in library %s", item.Name, item.LibraryID)
		}
		seen[key] = true
	}
	return all, nil
}

// ParseWidgetTypeManifests strictly parses one or more WidgetType documents.
func ParseWidgetTypeManifests(data []byte) ([]WidgetTypeManifest, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var items []WidgetTypeManifest
	for docIndex := 1; ; docIndex++ {
		var node yaml.Node
		err := decoder.Decode(&node)
		if errors.Is(err, io.EOF) {
			return items, nil
		}
		if err != nil {
			return nil, fmt.Errorf("document %d: %w", docIndex, err)
		}
		if isEmptyDocument(&node) {
			continue
		}
		var header manifestv0.Header
		if err := node.Decode(&header); err != nil {
			return nil, fmt.Errorf("document %d: %w", docIndex, err)
		}
		if err := checkAPIVersion(header.APIVersion); err != nil {
			return nil, err
		}
		if header.Kind != manifestv0.KindWidgetType {
			return nil, fmt.Errorf("unsupported kind %q, expected %q", header.Kind, manifestv0.KindWidgetType)
		}
		raw, err := yaml.Marshal(&node)
		if err != nil {
			return nil, err
		}
		strict := yaml.NewDecoder(bytes.NewReader(raw))
		strict.KnownFields(true)
		var doc manifestv0.WidgetTypeDocument
		if err := strict.Decode(&doc); err != nil {
			return nil, err
		}
		item, err := validateWidgetType(doc)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
}

func validateWidgetType(doc manifestv0.WidgetTypeDocument) (WidgetTypeManifest, error) {
	libID, err := uuid.Parse(doc.Metadata.Library)
	if err != nil || libID == uuid.Nil {
		return WidgetTypeManifest{}, errors.New("metadata.library must be a nonzero Library UUID")
	}
	name, err := library.NewWidgetTypeName(doc.Metadata.Name)
	if err != nil {
		return WidgetTypeManifest{}, fmt.Errorf("invalid metadata.name: %w", err)
	}
	spec := doc.Spec
	if spec.HtmlTemplate == nil || spec.Script == nil || spec.ScriptLanguage == nil || spec.DefaultWidth == nil || spec.DefaultHeight == nil || spec.InputPorts == nil {
		return WidgetTypeManifest{}, errors.New("spec requires htmlTemplate, script, scriptLanguage, defaultWidth, defaultHeight, and inputPorts")
	}
	if _, err := library.NewHtmlTemplate(*spec.HtmlTemplate); err != nil {
		return WidgetTypeManifest{}, fmt.Errorf("invalid spec.htmlTemplate: %w", err)
	}
	if _, err := library.NewScript(*spec.Script); err != nil {
		return WidgetTypeManifest{}, fmt.Errorf("invalid spec.script: %w", err)
	}
	lang, err := library.NewScriptLanguage(*spec.ScriptLanguage)
	if err != nil {
		return WidgetTypeManifest{}, fmt.Errorf("invalid spec.scriptLanguage: %w", err)
	}
	size, err := library.NewSize(*spec.DefaultWidth, *spec.DefaultHeight)
	if err != nil {
		return WidgetTypeManifest{}, fmt.Errorf("invalid default size: %w", err)
	}
	ports := make([]manifestv0.WidgetPort, len(*spec.InputPorts))
	seen := make(map[string]bool, len(ports))
	for i, port := range *spec.InputPorts {
		portName, err := library.NewInputPortName(port.Name)
		if err != nil {
			return WidgetTypeManifest{}, fmt.Errorf("invalid inputPorts[%d].name: %w", i, err)
		}
		if seen[portName.String()] {
			return WidgetTypeManifest{}, fmt.Errorf("duplicate input port %q", portName.String())
		}
		seen[portName.String()] = true
		hint, err := library.NewPortTypeHint(port.TypeHint)
		if err != nil {
			return WidgetTypeManifest{}, fmt.Errorf("invalid inputPorts[%d].typeHint: %w", i, err)
		}
		ports[i] = manifestv0.WidgetPort{Name: portName.String(), Description: port.Description, TypeHint: hint.String()}
	}
	if strings.TrimSpace(name.String()) == "" {
		return WidgetTypeManifest{}, errors.New("metadata.name is required")
	}
	return WidgetTypeManifest{LibraryID: libID, Name: name.String(), HtmlTemplate: *spec.HtmlTemplate, Script: *spec.Script, ScriptLanguage: lang.String(), DefaultWidth: size.Width(), DefaultHeight: size.Height(), InputPorts: ports}, nil
}
