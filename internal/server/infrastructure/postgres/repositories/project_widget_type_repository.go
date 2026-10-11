package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/kipitix/growscada/internal/server/domain/id"
	"github.com/kipitix/growscada/internal/server/domain/library"
	"github.com/kipitix/growscada/internal/server/domain/version"
)

// projectWidgetTypeRepository exposes only the immutable WidgetTypes from a
// project's pinned Library Releases. It is read-only by design.
type projectWidgetTypeRepository struct {
	db        *sql.DB
	projectID uuid.UUID
}

func NewProjectWidgetTypeRepository(db *sql.DB, projectID uuid.UUID) library.WidgetTypeRepository {
	return projectWidgetTypeRepository{db: db, projectID: projectID}
}
func (r projectWidgetTypeRepository) NextID() id.ID[library.WidgetType] {
	return id.NewID[library.WidgetType]()
}
func (r projectWidgetTypeRepository) Save(context.Context, library.WidgetType) (library.WidgetType, error) {
	return nil, errors.New("pinned library releases are read-only")
}
func (r projectWidgetTypeRepository) Delete(context.Context, library.WidgetType) error {
	return errors.New("pinned library releases are read-only")
}
func (r projectWidgetTypeRepository) FindByID(ctx context.Context, typeID id.ID[library.WidgetType]) (library.WidgetType, error) {
	items, err := r.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID() == typeID {
			return item, nil
		}
	}
	return nil, library.ErrWidgetTypeNotFound
}
func (r projectWidgetTypeRepository) FindAll(ctx context.Context) ([]library.WidgetType, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT r.content FROM project_library_pins p JOIN library_releases r ON r.library_id=p.library_id AND r.id=p.release_id WHERE p.project_id=$1 ORDER BY p.library_id`, r.projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []library.WidgetType{}
	for rows.Next() {
		var content []byte
		if err := rows.Scan(&content); err != nil {
			return nil, err
		}
		var snapshot struct {
			WidgetTypes []struct {
				ID             uuid.UUID `json:"id"`
				Name           string    `json:"name"`
				HTMLTemplate   string    `json:"html_template"`
				Script         string    `json:"script"`
				ScriptLanguage string    `json:"script_language"`
				DefaultWidth   int       `json:"default_width"`
				DefaultHeight  int       `json:"default_height"`
				InputPorts     []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					TypeHint    string `json:"type_hint"`
				} `json:"input_ports"`
				Version int `json:"version"`
			} `json:"widget_types"`
		}
		if err := json.Unmarshal(content, &snapshot); err != nil {
			return nil, fmt.Errorf("decode pinned Library Release: %w", err)
		}
		for _, item := range snapshot.WidgetTypes {
			wt, err := reconstructReleasedWidgetType(item.ID, item.Name, item.HTMLTemplate, item.Script, item.ScriptLanguage, item.DefaultWidth, item.DefaultHeight, item.InputPorts, item.Version)
			if err != nil {
				return nil, err
			}
			result = append(result, wt)
		}
	}
	return result, rows.Err()
}

func reconstructReleasedWidgetType(rawID uuid.UUID, name, html, script, language string, width, height int, rawPorts []struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TypeHint    string `json:"type_hint"`
}, number int) (library.WidgetType, error) {
	n, err := library.NewWidgetTypeName(name)
	if err != nil {
		return nil, err
	}
	h, err := library.NewHtmlTemplate(html)
	if err != nil {
		return nil, err
	}
	s, err := library.NewScript(script)
	if err != nil {
		return nil, err
	}
	lang, err := library.NewScriptLanguage(language)
	if err != nil {
		return nil, err
	}
	size, err := library.NewSize(width, height)
	if err != nil {
		return nil, err
	}
	ports := make([]library.InputPort, 0, len(rawPorts))
	for _, raw := range rawPorts {
		pn, err := library.NewInputPortNameFromStorage(raw.Name)
		if err != nil {
			return nil, err
		}
		hint, err := library.NewPortTypeHint(raw.TypeHint)
		if err != nil {
			return nil, err
		}
		ports = append(ports, library.NewInputPort(pn, raw.Description, hint))
	}
	v, err := version.New(version.WithNumber[library.WidgetType](number))
	if err != nil {
		return nil, err
	}
	return library.ReconstituteWidgetType(id.NewID(id.IDWithUUID[library.WidgetType](rawID)), n, h, s, lang, size, ports, v)
}

var _ library.WidgetTypeRepository = projectWidgetTypeRepository{}
