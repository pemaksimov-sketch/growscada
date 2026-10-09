package uiutil

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
	"strings"
	"testing"
)

func TestSceneBackgroundPreservesSandboxAndEscapesAttribute(t *testing.T) {
	html := app.HTMLString(SceneBackground(`<div title='" onload="alert(1)'>background</div><script>alert(2)</script>`))
	if !strings.Contains(html, `sandbox=""`) {
		t.Fatal("missing restrictive sandbox", html)
	}
	if strings.Contains(html, `<script>`) || strings.Contains(html, ` onload="`) {
		t.Fatal("srcdoc escaped its attribute", html)
	}
}
