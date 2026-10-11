package project

import apiv0 "github.com/kipitix/growscada/contract/api/v0"

// UI state uses the public contract directly; no second JSON schema is kept.
type portBindingDTO = apiv0.PortBinding
type widgetTypeItem = apiv0.WidgetTypeResponse
type getWidgetTypesResponse = apiv0.GetWidgetTypesResponse
type sceneItem = apiv0.SceneResponse
type getScenesResponse = apiv0.GetScenesResponse
type createSceneResponse = apiv0.CreateSceneResponse
type updateSceneResponse = apiv0.UpdateSceneResponse
type positionDTO = apiv0.PositionResponse
type sizeDTO = apiv0.SizeResponse
type originDTO = apiv0.OriginResponse
type rotationDTO = apiv0.RotationResponse
type transformMatrixDTO = apiv0.TransformMatrixResponse
type widgetItem = apiv0.WidgetResponse
type getWidgetsResponse = apiv0.GetWidgetsResponse
type createWidgetResponse = apiv0.CreateWidgetResponse
type updateWidgetResponse = apiv0.UpdateWidgetResponse
type tagItem = apiv0.TagResponse
type getTagsResponse = apiv0.GetTagsResponse
type createTagResponse = apiv0.CreateTagResponse
