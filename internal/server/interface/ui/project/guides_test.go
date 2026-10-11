package project

import (
	"math"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestGuides_SceneAndOtherWidgetAlignmentExcludesSelf(t *testing.T) {
	moving := widgetItem{ID: uuid.New(), Position: positionDTO{X: 2, Y: 103}, Size: sizeDTO{Width: 100, Height: 50}}
	other := widgetItem{ID: uuid.New(), Position: positionDTO{X: 300, Y: 100}, Size: sizeDTO{Width: 100, Height: 50}}
	guides := findGuides(moving, []widgetItem{moving, other}, 800, 600)
	if !slices.Equal(guides.x, []float64{0}) || !slices.Equal(guides.y, []float64{100, 125, 150}) {
		t.Fatalf("wrong alignment guides: %+v", guides)
	}
	if moving.Position.X != 2 || moving.Position.Y != 103 {
		t.Fatal("guides changed geometry")
	}
	moving.Position.X, moving.Position.Y = 350, 275
	guides = findGuides(moving, []widgetItem{moving}, 800, 600)
	if !slices.Contains(guides.x, 400) || !slices.Contains(guides.y, 300) {
		t.Fatalf("missing scene center: %+v", guides)
	}
}

func TestGuides_RotatedBoundsRespectOrigin(t *testing.T) {
	w := widgetItem{Position: positionDTO{X: 100, Y: 200}, Size: sizeDTO{Width: 120, Height: 60}, Origin: originDTO{X: .5, Y: .5}, Rotation: rotationDTO{Degrees: 90}}
	b := boundsOf(w)
	want := canvasBounds{130, 170, 190, 290}
	if math.Abs(b.left-want.left) > 1e-9 || math.Abs(b.top-want.top) > 1e-9 || math.Abs(b.right-want.right) > 1e-9 || math.Abs(b.bottom-want.bottom) > 1e-9 {
		t.Fatalf("rotated bounds: got %+v want %+v", b, want)
	}
	if got := matchingGuides([]float64{6}, []float64{0, 20}); len(got) != 0 {
		t.Fatalf("outside tolerance: %v", got)
	}
	if got := matchingGuides([]float64{5}, []float64{0, 0, 0}); !slices.Equal(got, []float64{0}) {
		t.Fatalf("duplicate guides: %v", got)
	}
}
