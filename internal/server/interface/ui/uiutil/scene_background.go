package uiutil

import "github.com/maxence-charriere/go-app/v10/pkg/app"

// SceneBackground renders static scene HTML in a script-free sandbox. It
// never receives pointer input, leaving canvas interaction to its owner.
func SceneBackground(backgroundHTML string) app.UI {
	return app.IFrame().
		Attr("sandbox", "").
		Attr("title", "Scene background").
		Attr("tabindex", "-1").
		Attr("aria-hidden", "true").
		Attr("srcdoc", `<!DOCTYPE html><html><head><style>html,body{margin:0;padding:0;overflow:hidden}</style></head><body>`+backgroundHTML+`</body></html>`).
		Style("position", "absolute").
		Style("inset", "0").
		Style("width", "100%").
		Style("height", "100%").
		Style("border", "none").
		Style("pointer-events", "none").
		Style("display", "block")
}
