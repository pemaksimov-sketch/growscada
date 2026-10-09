package uiutil

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"html"
)

// SceneBackground renders static scene HTML in a script-free sandbox. It
// never receives pointer input, leaving canvas interaction to its owner.
func SceneBackground(backgroundHTML string) app.UI {
	// go-app drops empty string attributes, including sandbox="". Raw HTML
	// preserves this security boundary; escape srcdoc as an attribute value.
	srcdoc := `<!DOCTYPE html><html><head><style>html,body{margin:0;padding:0;overflow:hidden}</style></head><body>` + backgroundHTML + `</body></html>`
	return app.Raw(`<iframe sandbox="" title="Scene background" tabindex="-1" aria-hidden="true" srcdoc="` + html.EscapeString(srcdoc) + `" style="position:absolute;inset:0;width:100%;height:100%;border:none;pointer-events:none;display:block"></iframe>`)
}
