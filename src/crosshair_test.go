package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var (
	green = [4]byte{0, 255, 0, 255}
	red   = [4]byte{0, 0, 255, 255}
	black = [4]byte{0, 0, 0, 255}
	clear = [4]byte{}
)

func (img Image) at(x, y int) [4]byte {
	var pixel [4]byte
	copy(pixel[:], img.Pix[(y*img.Side+x)*4:])
	return pixel
}

func expectPixel(t *testing.T, img Image, x, y int, want [4]byte) {
	t.Helper()
	if got := img.at(x, y); got != want {
		t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func TestDefaultCrosshair(t *testing.T) {
	img := Render(DefaultConfig())
	if img.Side != 28 {
		t.Fatalf("side = %d, want 28", img.Side)
	}
	expectPixel(t, img, 14, 14, clear) // The gap in the middle.
	expectPixel(t, img, 20, 14, green) // Right arm.
	expectPixel(t, img, 20, 15, black) // Its outline.
	expectPixel(t, img, 20, 11, clear) // Beyond the outline.
	expectPixel(t, img, 0, 0, clear)   // Corner.
}

func TestEveryShapeIsCentredAndFitsTheWindow(t *testing.T) {
	for length := 0; length <= 6; length++ {
		for gap := 0; gap <= 5; gap++ {
			for thickness := 0; thickness <= 5; thickness++ {
				for dot := 0; dot <= 5; dot++ {
					for outline := 0; outline <= 2; outline++ {
						c := DefaultConfig()
						c.Length, c.Gap, c.Thickness, c.DotSize, c.Outline = length, gap, thickness, dot, outline
						img := Render(c)
						name := fmt.Sprintf("%+v", c)
						last := img.Side - 1
						for i := 0; i <= last; i++ {
							for _, edge := range [][2]int{{i, 0}, {i, last}, {0, i}, {last, i}} {
								if img.at(edge[0], edge[1]) != clear {
									t.Fatalf("%s: touches the window edge at %v", name, edge)
								}
							}
						}
						if hasArms(c) && dot > 0 && dot%2 != thickness%2 {
							continue // Can't be centred exactly: see windowSide.
						}
						for y := 0; y <= last; y++ {
							for x := 0; x <= last; x++ {
								if img.at(x, y) != img.at(last-x, y) || img.at(x, y) != img.at(x, last-y) {
									t.Fatalf("%s: not symmetrical at (%d,%d)", name, x, y)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestCentreFilledWhenThereIsNoGap(t *testing.T) {
	for _, thickness := range []int{1, 2, 3, 4} {
		c := DefaultConfig()
		c.Gap, c.Thickness = 0, thickness
		img := Render(c)
		expectPixel(t, img, img.Side/2, img.Side/2, green)
	}
}

func TestSkippedTilesAreReallyEmpty(t *testing.T) {
	configs := []Config{}
	for _, list := range [][]Crosshair{presets, bases} {
		for _, item := range list {
			configs = append(configs, item.Config, item.Config.Scaled(3))
		}
	}
	for index, frame := range append(frameNames, "") {
		c := DefaultConfig()
		c.Frame, c.FrameSize, c.FrameThickness, c.FrameGaps = frame, 30, 2, index
		c.Outline, c.Glow, c.Rotation, c.RoundEnds = index%3, index*2, index*15, index%2 == 0
		configs = append(configs, c)
	}
	for _, c := range configs {
		side := windowSide(c)
		every := make([]byte, side*side*4)
		painter := newPainter(c, side)
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				painter.paint(x, y, every[(y*side+x)*4:])
			}
		}
		if !bytes.Equal(every, Render(c).Pix) {
			t.Errorf("%+v: skipping tiles changed the picture", c)
		}
	}
}

func alphaAt(img Image, x, y int) byte { return img.Pix[(y*img.Side+x)*4+3] }

func colorAt(img Image, x, y int) string {
	pixel := img.Pix[(y*img.Side+x)*4:]
	return fmt.Sprintf("#%02X%02X%02X", pixel[2], pixel[1], pixel[0])
}

func drawn(strokes ...Stroke) Config {
	return Config{Color: "#FFFFFF", OutlineColor: "#000000", Opacity: 1, Strokes: strokes}
}

func TestDrawnLinesFitAndKeepTheirColor(t *testing.T) {
	c := drawn(Stroke{Color: "#00FF00", Width: 4, Points: []float64{-30, -30, 30, 30}})
	img := Render(c)
	center := img.Side / 2
	if colorAt(img, center+20, center+20) != "#00FF00" || alphaAt(img, center+20, center+20) != 255 {
		t.Errorf("the line wasn't drawn: %s", colorAt(img, center+20, center+20))
	}
	if alphaAt(img, center+20, center-20) != 0 {
		t.Error("something was drawn away from the line")
	}
	for _, corner := range [][2]int{{0, 0}, {img.Side - 1, img.Side - 1}} {
		if alphaAt(img, corner[0], corner[1]) != 0 {
			t.Error("the line reached the edge of the window")
		}
	}
}

func TestASingleClickDrawsADot(t *testing.T) {
	img := Render(drawn(Stroke{Color: "#FF0000", Width: 6, Points: []float64{10, 0}}))
	center := img.Side / 2
	if colorAt(img, center+10, center) != "#FF0000" || alphaAt(img, center+10, center) != 255 {
		t.Error("the dot is missing")
	}
	if alphaAt(img, center+16, center) != 0 {
		t.Error("the dot is too large")
	}
}

func TestDrawnLinesGetOutlineGlowAndRotation(t *testing.T) {
	line := Stroke{Color: "#00FF00", Width: 2, Points: []float64{-20, 0, 20, 0}}
	c := drawn(line)
	c.Outline, c.OutlineColor = 2, "#FF0000"
	img := Render(c)
	center := img.Side / 2
	if colorAt(img, center, center+2) != "#FF0000" {
		t.Errorf("no outline: %s", colorAt(img, center, center+2))
	}
	c = drawn(line)
	c.Rotation = 90
	img = Render(c)
	center = img.Side / 2
	if alphaAt(img, center, center+15) == 0 || alphaAt(img, center+15, center) != 0 {
		t.Error("the drawing wasn't turned")
	}
}

func TestScalingDrawingsRedrawsAndLeavesTheOriginalAlone(t *testing.T) {
	c := drawn(Stroke{Color: "#FFFFFF", Width: 2, Points: []float64{-10, 0, 10, 0}})
	c.Merged = []Config{{Color: "#FF0000", OutlineColor: "#000000", Opacity: 1, DotSize: 4, OffsetX: 5}}
	big := c.Scaled(3)
	if c.Strokes[0].Points[0] != -10 || c.Strokes[0].Width != 2 || c.Merged[0].OffsetX != 5 || c.Merged[0].DotSize != 4 {
		t.Fatalf("scaling changed the original: %+v", c)
	}
	if big.Strokes[0].Points[0] != -30 || big.Strokes[0].Width != 6 || big.Merged[0].OffsetX != 15 || big.Merged[0].DotSize != 12 {
		t.Fatalf("scaled wrongly: %+v", big)
	}
	if side := windowSide(big); side <= windowSide(c)*2 {
		t.Errorf("window didn't grow: %d", side)
	}
}

func TestDrawingsAreChecked(t *testing.T) {
	load := func(strokes string) error {
		_, err := ParseConfig([]byte(`{"strokes":` + strokes + `}`))
		return err
	}
	if err := load(`[{"color":"#FF0000","width":2,"points":[1,2,3,4.5]}]`); err != nil {
		t.Errorf("a good line was refused: %v", err)
	}
	for name, strokes := range map[string]string{
		"bad color":  `[{"color":"red","width":2,"points":[1,2]}]`,
		"odd points": `[{"color":"#FF0000","width":2,"points":[1,2,3]}]`,
		"no points":  `[{"color":"#FF0000","width":2,"points":[]}]`,
		"too far":    `[{"color":"#FF0000","width":2,"points":[9999,0]}]`,
	} {
		if load(strokes) == nil {
			t.Errorf("%s wasn't refused", name)
		}
	}
	many := make([]Stroke, maxStrokes+1)
	for index := range many {
		many[index] = Stroke{Color: "#FF0000", Width: 1, Points: []float64{0, 0}}
	}
	data, _ := json.Marshal(many)
	if load(string(data)) == nil {
		t.Error("too many lines wasn't refused")
	}
	long := make([]float64, (maxStrokePoints+1)*2)
	data, _ = json.Marshal([]Stroke{{Color: "#FF0000", Width: 1, Points: long}})
	if load(string(data)) == nil {
		t.Error("too many points wasn't refused")
	}
	c, err := ParseConfig([]byte(`{"strokes":[{"color":"#FF0000","width":999,"points":[1,2]}]}`))
	if err != nil || c.Strokes[0].Width != maxStrokeWidth {
		t.Errorf("width wasn't limited: %v %v", c.Strokes, err)
	}
}

func TestMergedCrosshairsStackAndKeepTheirLook(t *testing.T) {
	cross := shape("#00FF00", 10, 4, 2, 0, 0)
	dot := shape("#FF0000", 0, 0, 0, 6, 0)
	cross.Merged = []Config{dot}
	img := Render(cross)
	center := img.Side / 2
	if colorAt(img, center, center) != "#FF0000" {
		t.Errorf("merged dot missing: %s", colorAt(img, center, center))
	}
	if colorAt(img, center+10, center) != "#00FF00" {
		t.Errorf("main crosshair missing: %s", colorAt(img, center+10, center))
	}

	// Each crosshair keeps its own outline.
	ringed := shape("#FF0000", 0, 0, 0, 6, 2)
	ringed.OutlineColor = "#0000FF"
	cross.Merged = []Config{ringed}
	img = Render(cross)
	center = img.Side / 2
	if colorAt(img, center, center+4) != "#0000FF" {
		t.Errorf("merged outline missing: %s", colorAt(img, center, center+4))
	}

	// Order
	over := shape("#FF0000", 10, 4, 2, 0, 0)
	base := shape("#00FF00", 10, 4, 2, 0, 0)
	base.Merged = []Config{over}
	img = Render(base)
	if got := colorAt(img, img.Side/2+10, img.Side/2); got != "#FF0000" {
		t.Errorf("on top: %s", got)
	}
	base.Merged[0].Behind = true
	img = Render(base)
	if got := colorAt(img, img.Side/2+10, img.Side/2); got != "#00FF00" {
		t.Errorf("behind: %s", got)
	}

	// Position and opacity
	base = shape("#00FF00", 4, 2, 2, 0, 0)
	moved := shape("#FF0000", 0, 0, 0, 6, 0)
	moved.OffsetX = 40
	base.Merged = []Config{moved}
	img = Render(base)
	if got := colorAt(img, img.Side/2+40, img.Side/2); got != "#FF0000" {
		t.Errorf("moved dot: %s", got)
	}
	base.Merged[0].Opacity = 0.5
	img = Render(base)
	if alpha := alphaAt(img, img.Side/2+40, img.Side/2); alpha < 120 || alpha > 135 {
		t.Errorf("faded dot alpha = %d", alpha)
	}
}

func TestMergingKeepsTheCentreAndTheOddOrEvenSize(t *testing.T) {
	base := shape("#00FF00", 8, 4, 3, 0, 0)
	alone := windowSide(base)
	wide := shape("#FF0000", 60, 4, 2, 0, 0)
	base.Merged = []Config{wide}
	if side := windowSide(base); side <= alone || side%2 != alone%2 {
		t.Errorf("window was %d, now %d", alone, side)
	}
	base.Merged[0].OffsetX = -30
	if side := windowSide(base); side < ownSide(wide)+60 {
		t.Errorf("moved crosshair doesn't fit: %d", side)
	}
}

func TestMergesAreChecked(t *testing.T) {
	c, err := ParseConfig([]byte(`{"merged":[{"color":"#FF0000","outline_color":"#000000","opacity":3,"length":99999,"merged":[{"color":"#FF0000","outline_color":"#000000"}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Merged[0].Merged != nil || c.Merged[0].Length != maxPixels || c.Merged[0].Opacity != 1 {
		t.Errorf("merged wasn't tidied: %+v", c.Merged[0])
	}
	if _, err := ParseConfig([]byte(`{"merged":[{"color":"nope","outline_color":"#000000"}]}`)); err == nil {
		t.Error("a bad merged crosshair wasn't refused")
	}
	four := strings.Repeat(`{"color":"#FF0000","outline_color":"#000000"},`, maxMerged+1)
	if _, err := ParseConfig([]byte(`{"merged":[` + strings.TrimSuffix(four, ",") + `]}`)); err == nil {
		t.Error("too many merged crosshairs wasn't refused")
	}
}

func TestDrawingsAndMergesSurviveTheSettingsFile(t *testing.T) {
	c := drawn(Stroke{Color: "#00FF00", Width: 3, Points: []float64{1, 2, 3, 4}})
	c.Merged = []Config{shape("#FF0000", 5, 2, 1, 0, 0)}
	c.Merged[0].Behind = true
	path := filepath.Join(t.TempDir(), configName)
	if err := SaveConfig(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(Render(c).Pix, Render(loaded).Pix) {
		t.Error("the saved crosshair draws differently")
	}
}

// Random settings go through the same checks as a share code or a hand-edited file.
func TestAnySettingsDrawSafely(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	number := func() float64 { return float64(r.Intn(2000)-1000) / 4 }
	stroke := func() Stroke {
		points := make([]float64, 2*(1+r.Intn(30)))
		for index := range points {
			points[index] = number()
			if index >= 2 && r.Intn(4) == 0 {
				points[index] = points[index-2] // Repeated points must not break a line.
			}
		}
		return Stroke{Color: "#FF0000", Width: float64(r.Intn(60)), Points: points}
	}
	config := func() Config {
		c := DefaultConfig()
		c.Length, c.Gap, c.Thickness, c.DotSize = r.Intn(90), r.Intn(50)-5, r.Intn(30), r.Intn(30)
		c.Outline, c.Glow, c.Rotation = r.Intn(8), r.Intn(40), r.Intn(900)-300
		c.Frame = append(frameNames, "")[r.Intn(len(frameNames)+1)]
		c.FrameSize, c.FrameThickness, c.FrameGaps = r.Intn(80), r.Intn(12), r.Intn(12)
		c.Opacity = r.Float64()
		for count := r.Intn(4); count > 0; count-- {
			c.Strokes = append(c.Strokes, stroke())
		}
		return c
	}
	for run := 0; run < 60; run++ {
		c := config()
		for count := r.Intn(4); count > 0; count-- {
			extra := config()
			extra.OffsetX, extra.OffsetY, extra.Behind = r.Intn(200)-100, r.Intn(200)-100, r.Intn(2) == 0
			c.Merged = append(c.Merged, extra)
		}
		data, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseConfig(data)
		if err != nil {
			t.Fatalf("settings were refused: %v", err)
		}
		img := Render(parsed)
		if len(img.Pix) != img.Side*img.Side*4 {
			t.Fatalf("picture size is wrong: %d", img.Side)
		}
	}
}

func TestDotDrawsOverArms(t *testing.T) {
	c := DefaultConfig()
	c.Color, c.Thickness, c.DotSize = "#FF0000", 3, 3
	img := Render(c)
	if img.Side%2 != 1 {
		t.Fatalf("side = %d, want odd", img.Side)
	}
	expectPixel(t, img, img.Side/2, img.Side/2, red)
}

func TestNothingToDraw(t *testing.T) {
	c := Config{Color: "#000000", OutlineColor: "#000000"}
	for _, value := range Render(c).Pix {
		if value != 0 {
			t.Fatal("expected an empty image")
		}
	}
}

func TestLargestSettingsFit(t *testing.T) {
	c := DefaultConfig()
	c.Length, c.Gap, c.Thickness, c.DotSize, c.Outline = maxPixels, maxPixels, maxPixels, maxPixels, maxPixels
	Render(c)
}

func TestParseKeepsDefaultsAndLimitsValues(t *testing.T) {
	c, err := ParseConfig([]byte(`{"color":"#ff0000","length":-5,"gap":999999,"opacity":7}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Color != "#ff0000" || c.Length != 0 || c.Gap != maxPixels || c.Opacity != 1 || c.Thickness != 2 {
		t.Errorf("unexpected config: %+v", c)
	}
}

func TestParseAcceptsByteOrderMark(t *testing.T) {
	if _, err := ParseConfig([]byte("\xef\xbb\xbf{\"gap\":1}")); err != nil {
		t.Error(err)
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	for _, bad := range []string{
		`{not json`, `[]`, ``, `{"gap":"wide"}`, `{"color":"notacolor"}`,
		`{"color":"#12345"}`, `{"outline_color":"#GGGGGG"}`,
	} {
		if _, err := ParseConfig([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestLoadCreatesSettingsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "crosshair_config.json")
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, DefaultConfig()) {
		t.Errorf("unexpected config: %+v", c)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(data, &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 11 {
		t.Errorf("got %d settings, want 11", len(keys))
	}
}

func preset(t *testing.T, id string) Config {
	t.Helper()
	for _, item := range presets {
		if item.ID == id {
			return item.Config
		}
	}
	t.Fatalf("no preset %s", id)
	return Config{}
}

func TestSoftEdgesPremultiply(t *testing.T) {
	c := preset(t, "ring-dot")
	c.Color, c.Outline = "#FFFFFF", 0
	img := Render(c)
	partial := -1
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 40 && img.Pix[i] < 215 {
			partial = i
			break
		}
	}
	if partial < 0 {
		t.Fatal("expected soft edges")
	}
	ready := img.Premultiplied()
	if ready.Pix[partial-1] != img.Pix[partial] && ready.Pix[partial-1] != img.Pix[partial]-1 {
		t.Errorf("colour %d doesn't match alpha %d", ready.Pix[partial-1], img.Pix[partial])
	}
}

func filled(img Image) (total int) {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] > 128 {
			total++
		}
	}
	return total
}

func TestEveryBaseDrawsInsideItsWindow(t *testing.T) {
	seen := map[string]bool{}
	for _, base := range bases {
		if seen[base.ID] {
			t.Errorf("duplicate base %s", base.ID)
		}
		seen[base.ID] = true
		if !slices.Contains(baseGroups, base.Category) {
			t.Errorf("%s: unknown group %q", base.ID, base.Category)
		}
		data, _ := json.Marshal(base.Config)
		parsed, err := ParseConfig(data)
		if err != nil {
			t.Fatalf("%s: %v", base.ID, err)
		}
		img := Render(parsed)
		if filled(img) == 0 {
			t.Errorf("%s: nothing was drawn", base.ID)
		}
		last := img.Side - 1
		for i := 0; i <= last; i++ {
			for _, edge := range [][2]int{{i, 0}, {i, last}, {0, i}, {last, i}} {
				if img.at(edge[0], edge[1])[3] != 0 {
					t.Fatalf("%s: touches the window edge at %v", base.ID, edge)
				}
			}
		}
	}
}

func TestEverySettingStaysInsideItsWindow(t *testing.T) {
	for _, frame := range append([]string{""}, frameNames...) {
		for _, rotation := range []int{0, 30, 45, 90} {
			for _, rounded := range []bool{false, true} {
				c := DefaultConfig()
				c.Frame, c.FrameSize, c.FrameThickness = frame, 20, 5
				c.Length, c.Gap, c.Thickness, c.Rotation, c.RoundEnds, c.Outline, c.Glow = 12, 14, 5, rotation, rounded, 3, 6
				img := Render(c)
				last := img.Side - 1
				for i := 0; i <= last; i++ {
					for _, edge := range [][2]int{{i, 0}, {i, last}, {0, i}, {last, i}} {
						if img.at(edge[0], edge[1])[3] != 0 {
							t.Fatalf("%+v touches the edge at %v", c, edge)
						}
					}
				}
			}
		}
	}
}

func TestRotationTurnsCrossIntoX(t *testing.T) {
	cross := preset(t, "classic-green")
	turned := cross
	turned.Rotation = 45
	img := Render(turned)
	center := img.Side / 2
	if alpha := img.at(center+6, center+6)[3]; alpha != 255 {
		t.Errorf("diagonal alpha = %d", alpha)
	}
	if alpha := img.at(center+9, center)[3]; alpha != 0 {
		t.Errorf("straight arm should be gone, alpha = %d", alpha)
	}
}

func TestHiddenArms(t *testing.T) {
	c := DefaultConfig()
	c.HideArms = "t"
	img := Render(c)
	center := img.Side / 2
	expectPixel(t, img, center, center-8, clear)
	expectPixel(t, img, center, center+8, green)
	expectPixel(t, img, center+8, center, green)
	c.HideArms = "trbl"
	if hasArms(c) {
		t.Error("all arms are hidden")
	}
	if filled(Render(c)) != 0 {
		t.Error("expected nothing to be drawn")
	}
}

func TestRoundEndsSoftenCorners(t *testing.T) {
	square := DefaultConfig()
	square.Thickness, square.Outline = 6, 0
	rounded := square
	rounded.RoundEnds = true
	if filled(Render(rounded)) >= filled(Render(square)) {
		t.Error("rounded arms should cover less")
	}
}

func TestDotShapes(t *testing.T) {
	c := DefaultConfig()
	c.Length, c.Outline, c.DotSize = 0, 0, 12
	square := Render(c)
	expectPixel(t, square, square.Side/2-5, square.Side/2-5, green)
	c.DotShape = "round"
	round := Render(c)
	if alpha := round.at(round.Side/2-6, round.Side/2-6)[3]; alpha != 0 {
		t.Errorf("round dot corner alpha = %d", alpha)
	}
	expectPixel(t, round, round.Side/2, round.Side/2, green)
	c.DotShape = "diamond"
	diamond := Render(c)
	if alpha := diamond.at(diamond.Side/2-5, diamond.Side/2-5)[3]; alpha != 0 {
		t.Errorf("diamond corner alpha = %d", alpha)
	}
}

func TestFramesAndTheirColours(t *testing.T) {
	c := DefaultConfig()
	c.Length, c.Outline = 0, 0
	c.Frame, c.FrameSize, c.FrameThickness, c.FrameColor = "circle", 20, 2, "#FF0000"
	c.DotSize, c.DotColor = 2, "#000000"
	img := Render(c)
	center := img.Side / 2
	if pixel := img.at(center+20, center); pixel[2] != 255 || pixel[3] < 240 {
		t.Errorf("frame pixel = %v, want red", pixel)
	}
	expectPixel(t, img, center, center, black)
	expectPixel(t, img, center+10, center, clear)
	if alpha := img.at(center+14, center+14)[3]; alpha == 0 {
		t.Error("expected the circle on the diagonal")
	}

	c.Frame, c.DotSize = "square", 0
	img = Render(c)
	center = img.Side / 2
	expectPixel(t, img, center+19, center+19, red)
	expectPixel(t, img, center-20, center+7, red)

	c.Frame = "brackets"
	img = Render(c)
	center = img.Side / 2
	expectPixel(t, img, center-20, center-19, red)
	expectPixel(t, img, center, center-20, clear)
}

func TestFrameGapsOpenTheShape(t *testing.T) {
	c := DefaultConfig()
	c.Length, c.Outline = 0, 0
	c.Frame, c.FrameSize, c.FrameThickness = "circle", 20, 2
	whole := Render(c)
	c.FrameGaps = 4
	broken := Render(c)
	center := broken.Side / 2
	expectPixel(t, whole, center, center-20, green)
	expectPixel(t, broken, center, center-20, clear)
	expectPixel(t, broken, center+20, center, clear)
	if alpha := broken.at(center+14, center+14)[3]; alpha == 0 {
		t.Error("the frame should remain between gaps")
	}
}

func TestGlowFadesOutward(t *testing.T) {
	c := DefaultConfig()
	c.Outline, c.Glow = 0, 8
	img := Render(c)
	center := img.Side / 2
	near := img.at(center+8, center-3)
	far := img.at(center+8, center-7)
	if near[3] == 0 || far[3] >= near[3] {
		t.Errorf("near %v, far %v", near, far)
	}
	if near[1] != 255 {
		t.Errorf("glow should use the crosshair colour: %v", near)
	}
}

func TestOlderSettingsDrawTheSame(t *testing.T) {
	saved := []byte(`{"color":"#00FF00","opacity":1,"length":8,"gap":4,"thickness":2,"dot_size":0,"outline":1,"outline_color":"#000000","offset_x":0,"offset_y":0,"screen":0}`)
	c, err := ParseConfig(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, DefaultConfig()) {
		t.Fatalf("config = %+v", c)
	}
	img := Render(c)
	greens, blacks := 0, 0
	for y := 0; y < img.Side; y++ {
		for x := 0; x < img.Side; x++ {
			switch img.at(x, y) {
			case green:
				greens++
			case black:
				blacks++
			case clear:
			default:
				t.Fatalf("soft pixel at (%d,%d): %v", x, y, img.at(x, y))
			}
		}
	}
	if greens != 64 || blacks != 96 {
		t.Errorf("%d green and %d black pixels", greens, blacks)
	}
}

func TestNewSettingsAreChecked(t *testing.T) {
	for _, bad := range []string{
		`{"frame":"blob"}`, `{"dot_shape":"star"}`, `{"hide_arms":"xyz"}`,
		`{"frame_color":"red"}`, `{"dot_color":"#12"}`,
	} {
		if _, err := ParseConfig([]byte(bad)); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
	c, err := ParseConfig([]byte(`{"rotation":-45,"glow":999,"frame_gaps":99,"frame_size":99999}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Rotation != 315 || c.Glow != maxGlow || c.FrameGaps != maxFrameGaps || c.FrameSize != maxPixels {
		t.Errorf("config = %+v", c)
	}
}

func TestScalingRedrawsInsteadOfStretching(t *testing.T) {
	small := Render(DefaultConfig())
	large := Render(DefaultConfig().Scaled(4))
	if filled(large) != filled(small)*16 {
		t.Errorf("%d pixels at 1x, %d at 4x", filled(small), filled(large))
	}
	for i := 3; i < len(large.Pix); i += 4 {
		if alpha := large.Pix[i]; alpha != 0 && alpha != 255 {
			t.Fatalf("straight lines should stay sharp, found alpha %d", alpha)
		}
	}
	// A stretched circle would have blocky steps. A redrawn one has fine, soft edges.
	ring := preset(t, "ring-dot")
	ring.Outline = 0
	edges := func(img Image) (total int) {
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 0 && img.Pix[i] < 255 {
				total++
			}
		}
		return total
	}
	if at1, at4 := edges(Render(ring)), edges(Render(ring.Scaled(4))); at4 < at1*3 {
		t.Errorf("%d soft pixels at 1x, %d at 4x", at1, at4)
	}
}

func TestPresetsScaleInProportion(t *testing.T) {
	for _, item := range presets {
		one, four := Render(item.Config), Render(item.Config.Scaled(4))
		ratio := float64(filled(four)) / float64(filled(one)+1)
		if ratio < 9 || ratio > 24 {
			t.Errorf("%s: coverage changed by %.1f times", item.ID, ratio)
		}
	}
}

func testPicture(t *testing.T, width, height int) string {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(picture.Pix); i += 4 {
		copy(picture.Pix[i:], []byte{255, 0, 0, 255})
	}
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(data.Bytes())
}

func TestPictureIsDrawnWhereItWasMoved(t *testing.T) {
	c := Config{Color: "#FFFFFF", OutlineColor: "#000000", Opacity: 1, Image: testPicture(t, 8, 8), ImageSize: 10, ImageX: 20, ImageY: -6}
	if err := c.clean(); err != nil {
		t.Fatal(err)
	}
	img := Render(c)
	pixel := func(x, y int) []byte { return img.Pix[(y*img.Side+x)*4 : (y*img.Side+x)*4+4] }
	middle := img.Side / 2
	if got := pixel(middle+20, middle-6); got[2] != 255 || got[0] != 0 || got[3] != 255 {
		t.Errorf("the picture's middle is %v", got)
	}
	if got := pixel(middle, middle); got[3] != 0 {
		t.Errorf("the picture should have moved off the centre, found %v", got)
	}
}

func TestPictureSitsUnderTheCrosshair(t *testing.T) {
	c := DefaultConfig()
	c.Outline = 0
	c.Image, c.ImageSize = testPicture(t, 4, 4), 60
	img := Render(c)
	at := (img.Side/2*img.Side + img.Side/2 + c.Gap + 2) * 4
	if got := img.Pix[at : at+4]; got[1] != 255 || got[2] != 0 {
		t.Errorf("the arm should be drawn over the picture, found %v", got)
	}
	if err := (&Config{Color: "#FFFFFF", OutlineColor: "#000000", Image: "not a picture"}).clean(); err == nil {
		t.Error("a broken picture should be refused")
	}
	scaled := c.Scaled(3)
	if scaled.ImageSize != 180 {
		t.Errorf("previews should scale the picture, got %d", scaled.ImageSize)
	}
}

func TestFilledShapesAndPictureEffects(t *testing.T) {
	c := Config{Color: "#FFFFFF", OutlineColor: "#000000", Opacity: 1, Strokes: []Stroke{{
		Color: "#FF0000", Width: 1, Fill: true, Points: []float64{-20, -20, 20, -20, 20, 20, -20, 20, -20, -20},
	}}}
	img := Render(c)
	middle := (img.Side/2*img.Side + img.Side/2) * 4
	if img.Pix[middle+3] != 255 {
		t.Error("a filled shape should cover its middle")
	}
	c.Strokes[0].Fill = false
	if img := Render(c); img.Pix[middle+3] != 0 {
		t.Error("an unfilled shape should be empty inside")
	}

	picture := Config{Color: "#00FF00", OutlineColor: "#0000FF", Opacity: 1, Image: testPicture(t, 8, 8), ImageSize: 20}
	plain := filled(Render(picture))
	picture.Outline = 3
	if outlined := filled(Render(picture)); outlined <= plain {
		t.Errorf("an outline should add to a picture: %d then %d", plain, outlined)
	}
	wide := Config{Color: "#FFFFFF", OutlineColor: "#000000", Opacity: 1, Image: testPicture(t, 40, 4), ImageSize: 40, Rotation: 90}
	img = Render(wide)
	at := func(x, y int) byte { return img.Pix[(y*img.Side+x)*4+3] }
	if at(img.Side/2, img.Side/2-15) == 0 || at(img.Side/2-15, img.Side/2) != 0 {
		t.Error("rotation should turn a picture")
	}
}
