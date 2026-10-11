// Package entityorigin identifies operational ownership. It is unrelated to
// scene.Origin, the geometric anchor point of a widget.
package entityorigin

import "fmt"

type Origin struct{ value uint8 }

var (
	Project = Origin{value: 1}
	Runtime = Origin{value: 2}
)

func Parse(value string) (Origin, error) {
	switch value {
	case "Project":
		return Project, nil
	case "Runtime":
		return Runtime, nil
	default:
		return Origin{}, fmt.Errorf("invalid entity origin %q", value)
	}
}
func (o Origin) IsValid() bool { return o == Project || o == Runtime }
func (o Origin) String() string {
	switch o {
	case Project:
		return "Project"
	case Runtime:
		return "Runtime"
	default:
		return "invalid"
	}
}
