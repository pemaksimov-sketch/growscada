package project

import (
	"fmt"
	"math"
	"slices"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

const guideTolerance = 5.0 // Canvas pixels; guides never change geometry.

type canvasBounds struct{ left, top, right, bottom float64 }

// boundsOf uses the same origin-aware transform as the widget canvas.
func boundsOf(w widgetItem) canvasBounds {
	a := w.Rotation.Degrees * math.Pi / 180
	c, s := math.Cos(a), math.Sin(a)
	ox, oy := w.Origin.X*float64(w.Size.Width), w.Origin.Y*float64(w.Size.Height)
	b := canvasBounds{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, corner := range [][2]float64{{0, 0}, {float64(w.Size.Width), 0}, {0, float64(w.Size.Height)}, {float64(w.Size.Width), float64(w.Size.Height)}} {
		x := w.Position.X + ox + c*(corner[0]-ox) - s*(corner[1]-oy)
		y := w.Position.Y + oy + s*(corner[0]-ox) + c*(corner[1]-oy)
		b.left, b.right = math.Min(b.left, x), math.Max(b.right, x)
		b.top, b.bottom = math.Min(b.top, y), math.Max(b.bottom, y)
	}
	return b
}

func (b canvasBounds) xs() []float64 { return []float64{b.left, (b.left + b.right) / 2, b.right} }
func (b canvasBounds) ys() []float64 { return []float64{b.top, (b.top + b.bottom) / 2, b.bottom} }

type alignmentGuides struct{ x, y []float64 }

func matchingGuides(moving, targets []float64) []float64 {
	var result []float64
	for _, target := range targets {
		if !slices.ContainsFunc(moving, func(value float64) bool { return math.Abs(value-target) <= guideTolerance }) {
			continue
		}
		if !slices.ContainsFunc(result, func(value float64) bool { return math.Abs(value-target) < 0.01 }) {
			result = append(result, target)
		}
	}
	slices.Sort(result)
	return result
}

func findGuides(moving widgetItem, widgets []widgetItem, width, height int) alignmentGuides {
	scene := canvasBounds{0, 0, float64(width), float64(height)}
	xs, ys := scene.xs(), scene.ys()
	for _, other := range widgets {
		if other.ID == moving.ID {
			continue
		}
		b := boundsOf(other)
		xs, ys = append(xs, b.xs()...), append(ys, b.ys()...)
	}
	b := boundsOf(moving)
	return alignmentGuides{matchingGuides(b.xs(), xs), matchingGuides(b.ys(), ys)}
}

func (p *Project) renderGuides(width, height int) []app.UI {
	if !p.showGuides {
		return nil
	}
	activeID := p.draggingWidgetID
	if activeID == "" {
		activeID = p.resizingWidgetID
	}
	if activeID == "" {
		return nil
	}
	w, found := p.findWidget(activeID)
	if !found {
		return nil
	}
	guides := findGuides(w, p.widgets, width, height)
	var lines []app.UI
	line := func(axis string, position float64) app.HTMLDiv {
		return app.Div().Class("scene-guide").Attr("aria-hidden", "true").Attr("data-axis", axis).
			Attr("data-position", fmt.Sprintf("%.2f", position)).Style("position", "absolute").
			Style("pointer-events", "none").Style("z-index", "10000")
	}
	for _, x := range guides.x {
		lines = append(lines, line("x", x).Style("left", fmt.Sprintf("%.2fpx", x)).Style("top", "0").
			Style("height", "100%").Style("border-left", "1px dashed var(--accent)"))
	}
	for _, y := range guides.y {
		lines = append(lines, line("y", y).Style("top", fmt.Sprintf("%.2fpx", y)).Style("left", "0").
			Style("width", "100%").Style("border-top", "1px dashed var(--accent)"))
	}
	return lines
}
