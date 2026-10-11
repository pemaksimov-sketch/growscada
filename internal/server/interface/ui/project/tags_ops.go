package project

import (
	"context"
	"github.com/google/uuid"
	apiv0 "github.com/kipitix/growscada/contract/api/v0"
	"github.com/kipitix/growscada/internal/server/interface/ui/uiutil"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func (p *Project) createTag(ctx app.Context) {
	input := apiv0.CreateTagRequest{Name: p.newTagName, Type: p.newTagType, Value: defaultTagValue(p.newTagType), Quality: "good"}
	uiutil.Request(p.tasks, func(ctx context.Context) (apiv0.CreateTagResponse, error) {
		return p.client.CreateTagResult(ctx, input)
	}, func(result apiv0.CreateTagResponse) {
		p.creatingTag = false
		p.newTagName = ""
		p.newTagType = ""
		p.selectedTagID = result.ID.String()
		p.tasks.Store("project:tagID", p.selectedTagID)
		p.loadTags(p.compoCtx)
	}, nil)
}
func (p *Project) deleteTag(ctx app.Context) {
	if p.selectedTagID == "" {
		return
	}
	deletedID := p.selectedTagID
	uiutil.Request(p.tasks, func(ctx context.Context) (struct{}, error) {
		id, err := uuid.Parse(deletedID)
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, p.client.DeleteTag(ctx, id)
	}, func(struct{}) {
		if p.selectedTagID == deletedID {
			p.selectedTagID = ""
			p.tasks.Store("project:tagID", "")
		}
		p.loadTags(p.compoCtx)
	}, nil)
}
func defaultTagValue(tagType string) string {
	switch tagType {
	case "boolean":
		return "false"
	case "integer":
		return "0"
	default:
		return ""
	}
}
