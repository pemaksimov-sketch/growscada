package livescene

import (
	"github.com/google/uuid"
	"reflect"
	"testing"

	"github.com/kipitix/growscada/internal/server/interface/ui/uidto"
)

func testWidgetType() WidgetType {
	return WidgetType{
		ID: uuid.MustParse("00000000-0000-4000-8000-000000000004"),
		InputPorts: []uidto.InputPortDTO{
			{Name: "speed", TypeHint: "integer"},
			{Name: "any", TypeHint: ""},
			{Name: "on", TypeHint: "boolean"},
		},
	}
}

func testWidget(bindings ...PortBinding) Widget {
	return Widget{ID: uuid.MustParse("00000000-0000-4000-8000-000000000003"), TypeID: uuid.MustParse("00000000-0000-4000-8000-000000000004"), PortBindings: bindings}
}

func TestWidgetInputs(t *testing.T) {
	tags := map[string]Tag{
		"00000000-0000-4000-8000-000000000013": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000013"), Type: "integer", Value: "42", Quality: string(QualityGood)},
		"00000000-0000-4000-8000-000000000014": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000014"), Type: "integer", Value: "7", Quality: string(QualityGood)},
	}
	w := testWidget(
		PortBinding{PortName: "speed", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000013")},
		PortBinding{PortName: "any", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000014")},
		PortBinding{PortName: "on", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000015")},
	)

	inputs := WidgetInputs(w, testWidgetType(), tags)

	if got := inputs["speed"].Value; got != int64(42) {
		t.Errorf("speed = %#v, want 42", got)
	}
	if got := inputs["any"].Value; got != int64(7) {
		t.Errorf("an \"any\" port takes its Tag's type: got %#v, want 7", got)
	}
	if got := inputs["on"].Value; got != false {
		t.Errorf("a port bound to a deleted Tag gets its default: got %#v, want false", got)
	}
}

func TestWidgetInputs_UnboundPortsGetDefaults(t *testing.T) {
	inputs := WidgetInputs(testWidget(), testWidgetType(), nil)

	if got := inputs["speed"].Value; got != int64(0) {
		t.Errorf("speed = %#v, want 0", got)
	}
	if got := inputs["any"].Value; got != nil {
		t.Errorf("any = %#v, want undefined", got)
	}
	if len(inputs) != 3 {
		t.Errorf("expected an input per port, got %v", inputs)
	}
}

func TestWidgetQuality(t *testing.T) {
	good := func(id string) Tag {
		return Tag{ID: uuid.MustParse(id), Type: "integer", Value: "1", Quality: string(QualityGood)}
	}
	tags := map[string]Tag{
		"00000000-0000-4000-8000-000000000017": good("00000000-0000-4000-8000-000000000017"), "00000000-0000-4000-8000-000000000018": good("00000000-0000-4000-8000-000000000018"), "00000000-0000-4000-8000-000000000019": good("00000000-0000-4000-8000-000000000019"),
		"00000000-0000-4000-8000-000000000020": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000020"), Type: "integer", Value: "1", Quality: string(QualityUncertain)},
		"00000000-0000-4000-8000-000000000012": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000012"), Type: "integer", Value: "1", Quality: string(QualityBad)},
	}
	bind := func(speed, anyTag, on string) Widget {
		return testWidget(
			PortBinding{PortName: "speed", TagID: uuid.MustParse(speed)},
			PortBinding{PortName: "any", TagID: uuid.MustParse(anyTag)},
			PortBinding{PortName: "on", TagID: uuid.MustParse(on)},
		)
	}

	cases := []struct {
		name      string
		w         Widget
		connected bool
		want      Quality
	}{
		{"all good", bind("00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000018", "00000000-0000-4000-8000-000000000019"), true, QualityGood},
		{"one uncertain", bind("00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000020", "00000000-0000-4000-8000-000000000019"), true, QualityUncertain},
		{"bad beats uncertain", bind("00000000-0000-4000-8000-000000000012", "00000000-0000-4000-8000-000000000020", "00000000-0000-4000-8000-000000000019"), true, QualityBad},
		{"deleted tag is bad", bind("00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000018", "00000000-0000-4000-8000-000000000021"), true, QualityBad},
		{"unbound port is bad", testWidget(PortBinding{PortName: "speed", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000017")}), true, QualityBad},
		{"disconnected is uncertain", bind("00000000-0000-4000-8000-000000000017", "00000000-0000-4000-8000-000000000018", "00000000-0000-4000-8000-000000000019"), false, QualityUncertain},
		{"disconnected keeps bad", bind("00000000-0000-4000-8000-000000000012", "00000000-0000-4000-8000-000000000018", "00000000-0000-4000-8000-000000000019"), false, QualityBad},
	}
	for _, c := range cases {
		if got := WidgetQuality(c.w, testWidgetType(), tags, c.connected); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestWidgetQuality_NoPorts(t *testing.T) {
	wt := WidgetType{ID: uuid.MustParse("00000000-0000-4000-8000-000000000004")}
	if got := WidgetQuality(testWidget(), wt, nil, true); got != QualityGood {
		t.Errorf("connected: got %s, want good", got)
	}
	if got := WidgetQuality(testWidget(), wt, nil, false); got != QualityUncertain {
		t.Errorf("disconnected: got %s, want uncertain", got)
	}
}

func TestWorst(t *testing.T) {
	cases := []struct{ a, b, want Quality }{
		{QualityGood, QualityGood, QualityGood},
		{QualityGood, QualityUncertain, QualityUncertain},
		{QualityBad, QualityUncertain, QualityBad},
		{QualityGood, "weird", QualityBad},
	}
	for _, c := range cases {
		if got := Worst(c.a, c.b); got != c.want {
			t.Errorf("Worst(%s, %s) = %s, want %s", c.a, c.b, got, c.want)
		}
	}
}

func TestSceneTags_KeepsOnlyBoundTags(t *testing.T) {
	tags := map[string]Tag{
		"00000000-0000-4000-8000-000000000013": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000013"), Value: "42"},
		"00000000-0000-4000-8000-000000000016": {ID: uuid.MustParse("00000000-0000-4000-8000-000000000016"), Value: "1"},
	}
	widgets := []Widget{
		testWidget(PortBinding{PortName: "speed", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000013")}),
		testWidget(PortBinding{PortName: "on", TagID: uuid.MustParse("00000000-0000-4000-8000-000000000015")}),
	}

	got := SceneTags(widgets, tags)

	if len(got) != 1 || got["00000000-0000-4000-8000-000000000013"].Value != "42" {
		t.Errorf("want only the existing bound Tag, got %+v", got)
	}

	// A change of an unbound Tag must leave the result equal, so the View is
	// not re-rendered.
	tags["00000000-0000-4000-8000-000000000016"] = Tag{ID: uuid.MustParse("00000000-0000-4000-8000-000000000016"), Value: "2"}
	if again := SceneTags(widgets, tags); !reflect.DeepEqual(again, got) {
		t.Errorf("an unbound Tag changed the result: %+v vs %+v", again, got)
	}
}
