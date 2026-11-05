// Vector drawing for crosshairs. Every part is a distance field: a function that
// says how far a point is from the part's edge, negative when inside. Smooth edges,
// outlines and glow then become one calculation that works for every shape, and
// parts that sit on whole pixels stay perfectly sharp.
package main

import (
	"math"
	"slices"
	"strings"
)

const (
	// diamondReach makes a diamond dot look about as large as a square of the same size.
	diamondReach = 0.68

	tileSide = 8 // How many pixels wide a block of the window Render can skip is.
)

// field returns the distance from a point to a shape's edge.
type field func(x, y float64) float64

// Scaled returns the crosshair drawn n times larger. Shapes are recalculated rather
// than stretched, so previews and high-resolution screens stay perfectly sharp.
func (c Config) Scaled(n int) Config {
	for _, size := range []*int{&c.Length, &c.Gap, &c.Thickness, &c.DotSize, &c.Outline, &c.Glow, &c.FrameSize, &c.FrameThickness} {
		*size *= n
	}
	strokes := slices.Clone(c.Strokes)
	for index, stroke := range strokes {
		stroke.Width *= float64(n)
		stroke.Points = slices.Clone(stroke.Points)
		for point := range stroke.Points {
			stroke.Points[point] *= float64(n)
		}
		strokes[index] = stroke
	}
	merged := slices.Clone(c.Merged)
	for index, extra := range merged {
		merged[index] = extra.Scaled(n)
		merged[index].OffsetX *= n
		merged[index].OffsetY *= n
	}
	c.Strokes, c.Merged = strokes, merged
	c.ImageSize, c.ImageX, c.ImageY = c.ImageSize*n, c.ImageX*n, c.ImageY*n
	return c
}

// layer is one coloured part of a crosshair. Later layers draw on top.
type layer struct {
	fields []field
	color  [4]byte
	box    *bounds // The area a drawn line stays inside, so it's quick to skip.
}

type bounds struct{ left, top, right, bottom float64 }

// reaches reports whether a point is within a distance of the box.
func (b *bounds) reaches(x, y, distance float64) bool {
	return x >= b.left-distance && x <= b.right+distance && y >= b.top-distance && y <= b.bottom+distance
}

func (l layer) distance(x, y float64) float64 {
	nearest := math.Inf(1)
	for _, part := range l.fields {
		nearest = min(nearest, part(x, y))
	}
	return nearest
}

type rect struct{ x0, y0, x1, y1 int }

func boxField(r rect, rounded bool) field {
	centerX, centerY := float64(r.x0+r.x1)/2, float64(r.y0+r.y1)/2
	halfWidth, halfHeight := float64(r.x1-r.x0)/2, float64(r.y1-r.y0)/2
	if !rounded {
		return func(x, y float64) float64 {
			return max(math.Abs(x-centerX)-halfWidth, math.Abs(y-centerY)-halfHeight)
		}
	}
	radius := min(halfWidth, halfHeight)
	return func(x, y float64) float64 {
		overX := max(math.Abs(x-centerX)-halfWidth+radius, 0)
		overY := max(math.Abs(y-centerY)-halfHeight+radius, 0)
		return math.Hypot(overX, overY) - radius
	}
}

func circleField(centerX, centerY, radius float64) field {
	return func(x, y float64) float64 {
		return math.Hypot(x-centerX, y-centerY) - radius
	}
}

func diamondField(centerX, centerY, radius float64) field {
	return func(x, y float64) float64 {
		return (math.Abs(x-centerX) + math.Abs(y-centerY) - radius) / math.Sqrt2
	}
}

func ringField(centerX, centerY, radius, width float64) field {
	return func(x, y float64) float64 {
		return math.Abs(math.Hypot(x-centerX, y-centerY)-radius) - width/2
	}
}

func squareRingField(centerX, centerY, radius, width float64) field {
	return func(x, y float64) float64 {
		return math.Abs(max(math.Abs(x-centerX), math.Abs(y-centerY))-radius) - width/2
	}
}

// segmentField is a straight line between two points.
func segmentField(x1, y1, x2, y2, width float64, rounded bool) field {
	length := math.Hypot(x2-x1, y2-y1)
	unitX, unitY := (x2-x1)/length, (y2-y1)/length
	half := width / 2
	return func(x, y float64) float64 {
		offsetX, offsetY := x-x1, y-y1
		along := offsetX*unitX + offsetY*unitY
		across := math.Abs(offsetY*unitX - offsetX*unitY)
		beyond := max(-along, along-length)
		if rounded {
			return math.Hypot(max(beyond, 0), across) - half
		}
		return max(across, beyond) - half
	}
}

// polygonFields draws the edges of a regular shape with its first corner on top.
func polygonFields(centerX, centerY, radius, width float64, corners int, rounded bool) []field {
	parts := make([]field, 0, corners)
	corner := func(index int) (float64, float64) {
		angle := 2*math.Pi*float64(index)/float64(corners) - math.Pi/2
		return centerX + radius*math.Cos(angle), centerY + radius*math.Sin(angle)
	}
	for index := 0; index < corners; index++ {
		x1, y1 := corner(index)
		x2, y2 := corner(index + 1)
		parts = append(parts, segmentField(x1, y1, x2, y2, width, rounded))
	}
	return parts
}

func rotatedField(part field, degrees, centerX, centerY float64) field {
	sin, cos := math.Sincos(degrees * math.Pi / 180)
	return func(x, y float64) float64 {
		offsetX, offsetY := x-centerX, y-centerY
		return part(centerX+offsetX*cos+offsetY*sin, centerY-offsetX*sin+offsetY*cos)
	}
}

// gappedField cuts evenly spaced openings in a shape, starting from the top.
func gappedField(part field, count int, width, centerX, centerY float64) field {
	step := 2 * math.Pi / float64(count)
	return func(x, y float64) float64 {
		offsetX, offsetY := x-centerX, y-centerY
		angle := math.Atan2(offsetY, offsetX) + math.Pi/2
		fromCut := angle - math.Round(angle/step)*step
		distance := math.Hypot(offsetX, offsetY) * math.Abs(math.Sin(fromCut))
		return max(part(x, y), width/2-distance)
	}
}

func hasArms(c Config) bool {
	return c.Length > 0 && c.Thickness > 0 && len(c.HideArms) < 4
}

// frameReach is how far a frame extends from the centre.
func frameReach(c Config) int {
	if c.Frame == "" {
		return 0
	}
	return c.FrameSize + c.FrameThickness
}

// strokeReach is how far drawn lines extend from the centre.
func strokeReach(c Config) int {
	reach := 0.0
	for _, stroke := range c.Strokes {
		for _, value := range stroke.Points {
			reach = max(reach, math.Abs(value)+stroke.Width/2)
		}
	}
	return int(math.Ceil(reach))
}

// windowSide returns the smallest window that fits the crosshair and anything merged
// with it, keeping the odd or even size of the main crosshair.
func windowSide(c Config) int {
	side := ownSide(c)
	for _, extra := range c.Merged {
		needed := ownSide(extra) + 2*max(abs(extra.OffsetX), abs(extra.OffsetY))
		if needed > side {
			side += (needed - side + 1) / 2 * 2
		}
	}
	return side
}

func abs(value int) int { return max(value, -value) }

// ownSide sizes one crosshair. Its odd or even size matches the arms so they can sit
// exactly in the middle. If a dot's size is odd and the arms' size is even, or the
// reverse, the dot sits half a pixel off centre, because a pixel can't be split.
func ownSide(c Config) int {
	reach := max(c.Gap+c.Length, (max(c.Thickness, c.DotSize)+1)/2, frameReach(c), strokeReach(c), imageReach(c))
	if c.DotShape == "diamond" {
		reach = max(reach, int(math.Ceil(float64(c.DotSize)*diamondReach)))
	}
	if c.Rotation%90 != 0 {
		// A turned crosshair reaches further into the corners of the window.
		reach = int(math.Ceil(math.Sqrt2 * float64(reach)))
	}
	side := 2 * (reach + c.Outline + c.Glow + 1)
	switch {
	case hasArms(c):
		return side + c.Thickness%2
	case c.DotSize > 0:
		return side + c.DotSize%2
	case c.Frame != "":
		return side + c.FrameThickness%2
	}
	return side
}

// armRects returns the arms in window coordinates.
func armRects(c Config, side int) []rect {
	if !hasArms(c) {
		return nil
	}
	// low and high are the edges of the centre: the same line in an even-sized
	// window, and either side of the centre pixel in an odd one.
	low := side / 2
	high := side - low
	a := (side - c.Thickness) / 2
	b := a + c.Thickness
	before := low - c.Gap
	after := high + c.Gap
	arms := []struct {
		name string
		rect rect
	}{
		{"t", rect{a, before - c.Length, b, before}},
		{"b", rect{a, after, b, after + c.Length}},
		{"l", rect{before - c.Length, a, before, b}},
		{"r", rect{after, a, after + c.Length, b}},
	}
	var out []rect
	for _, arm := range arms {
		if !strings.Contains(c.HideArms, arm.name) {
			out = append(out, arm.rect)
		}
	}
	if c.Gap == 0 {
		out = append(out, rect{a, a, b, b}) // Closes the middle of a solid cross.
	}
	return out
}

func dotFields(c Config, side int) []field {
	if c.DotSize <= 0 {
		return nil
	}
	center, size := float64(side)/2, float64(c.DotSize)
	switch c.DotShape {
	case "round":
		return []field{circleField(center, center, size/2)}
	case "diamond":
		return []field{diamondField(center, center, size*diamondReach)}
	}
	a := (side - c.DotSize) / 2
	return []field{boxField(rect{a, a, a + c.DotSize, a + c.DotSize}, false)}
}

func frameFields(c Config, side int) []field {
	if c.Frame == "" || c.FrameSize <= 0 || c.FrameThickness <= 0 {
		return nil
	}
	center := float64(side) / 2
	size, width := float64(c.FrameSize), float64(c.FrameThickness)
	var parts []field
	switch c.Frame {
	case "circle":
		parts = []field{ringField(center, center, size, width)}
	case "square":
		parts = []field{squareRingField(center, center, size, width)}
	case "diamond":
		parts = polygonFields(center, center, size, width, 4, c.RoundEnds)
	case "triangle":
		parts = polygonFields(center, center, size, width, 3, c.RoundEnds)
	case "hexagon":
		parts = polygonFields(center, center, size, width, 6, c.RoundEnds)
	case "chevron":
		// The tip sits on the aiming point.
		for _, side := range []float64{-1, 1} {
			parts = append(parts, segmentField(center, center, center+side*size, center+size*0.72, width, c.RoundEnds))
		}
	case "brackets":
		reach := size * 0.5
		for _, signX := range []float64{-1, 1} {
			for _, signY := range []float64{-1, 1} {
				cornerX, cornerY := center+signX*size, center+signY*size
				parts = append(parts,
					segmentField(cornerX, cornerY, cornerX-signX*reach, cornerY, width, c.RoundEnds),
					segmentField(cornerX, cornerY, cornerX, cornerY-signY*reach, width, c.RoundEnds))
			}
		}
	}
	if c.FrameGaps > 0 {
		for index, part := range parts {
			parts[index] = gappedField(part, c.FrameGaps, width*2, center, center)
		}
	}
	return parts
}

// fillField is the inside of a closed line. Inside counts as a negative distance, and a
// point is inside if a line from it to the edge of the window crosses the shape an odd number of times.
func fillField(points []float64, center float64) field {
	corners := slices.Clone(points)
	for index := range corners {
		corners[index] += center
	}
	count := len(corners) / 2
	return func(x, y float64) float64 {
		nearest, inside := math.Inf(1), false
		for index := range count {
			next := (index + 1) % count
			fromX, fromY, toX, toY := corners[index*2], corners[index*2+1], corners[next*2], corners[next*2+1]
			alongX, alongY := toX-fromX, toY-fromY
			length := alongX*alongX + alongY*alongY
			share := 0.0
			if length > 0 {
				share = min(max(((x-fromX)*alongX+(y-fromY)*alongY)/length, 0), 1)
			}
			nearest = min(nearest, math.Hypot(x-fromX-share*alongX, y-fromY-share*alongY))
			if (fromY > y) != (toY > y) && x < fromX+(y-fromY)/(toY-fromY)*alongX {
				inside = !inside
			}
		}
		if inside {
			return -nearest
		}
		return nearest
	}
}

// strokeLayer turns a drawn line into a part of the crosshair.
func strokeLayer(stroke Stroke, center float64, fallback [4]byte) layer {
	x, y := center+stroke.Points[0], center+stroke.Points[1]
	box := bounds{x, y, x, y}
	var parts []field
	for index := 2; index+1 < len(stroke.Points); index += 2 {
		nextX, nextY := center+stroke.Points[index], center+stroke.Points[index+1]
		if nextX == x && nextY == y {
			continue
		}
		parts = append(parts, segmentField(x, y, nextX, nextY, stroke.Width, true))
		x, y = nextX, nextY
		box.left, box.right = min(box.left, x), max(box.right, x)
		box.top, box.bottom = min(box.top, y), max(box.bottom, y)
	}
	if len(parts) == 0 {
		parts = []field{circleField(x, y, stroke.Width/2)}
	}
	if stroke.Fill && len(stroke.Points) >= 6 {
		parts = append(parts, fillField(stroke.Points, center))
	}
	half := stroke.Width / 2
	box.left, box.top, box.right, box.bottom = box.left-half, box.top-half, box.right+half, box.bottom+half
	return layer{fields: parts, color: colorOr(stroke.Color, fallback), box: &box}
}

func colorOr(text string, fallback [4]byte) [4]byte {
	if parsed, err := parseColor(text); err == nil {
		return parsed
	}
	return fallback
}

// layers lists the crosshair's parts from bottom to top.
func layers(c Config, side int) []layer {
	color := colorOr(c.Color, [4]byte{})
	var arms []field
	for _, arm := range armRects(c, side) {
		arms = append(arms, boxField(arm, c.RoundEnds))
	}
	var parts []layer
	for _, part := range []layer{
		{fields: frameFields(c, side), color: colorOr(c.FrameColor, color)},
		{fields: arms, color: color},
		{fields: dotFields(c, side), color: colorOr(c.DotColor, color)},
	} {
		if len(part.fields) > 0 {
			parts = append(parts, part)
		}
	}
	for _, stroke := range c.Strokes {
		parts = append(parts, strokeLayer(stroke, float64(side)/2, color))
	}
	if c.Rotation != 0 {
		center := float64(side) / 2
		for number, part := range parts {
			parts[number].box = nil // A box no longer fits once the line is turned.
			for index, each := range part.fields {
				part.fields[index] = rotatedField(each, float64(c.Rotation), center, center)
			}
		}
	}
	return parts
}

// mixer stacks see-through colours on one pixel.
type mixer struct {
	color [3]float64
	alpha float64
}

func (m *mixer) add(color [4]byte, cover float64) {
	if cover <= 0 {
		return
	}
	below := m.alpha * (1 - cover)
	total := cover + below
	for channel := range m.color {
		m.color[channel] = (float64(color[channel])*cover + m.color[channel]*below) / total
	}
	m.alpha = total
}

func (m *mixer) write(pixel []byte) {
	if m.alpha <= 0 {
		return
	}
	for channel := range m.color {
		pixel[channel] = byte(min(m.color[channel]+0.5, 255))
	}
	pixel[3] = byte(m.alpha*255 + 0.5)
}

// coverage turns a distance into how much of a pixel a shape fills.
func coverage(distance float64) float64 {
	switch {
	case distance >= 0.5:
		return 0
	case distance <= -0.5:
		return 1
	}
	return 0.5 - distance
}

// painter colours the pixels of one crosshair.
type painter struct {
	parts        []layer   // Everything in the crosshair.
	near         []layer   // The parts close enough to matter in the current tile.
	kept         [][]field // Reused storage for near, one list for each part.
	distances    []float64 // How far the last point measured is from each near part.
	glowColor    [4]byte
	outlineColor [4]byte
	outline      float64
	glow         float64
	reach        float64 // Points this far from every part stay empty.
}

func newPainter(c Config, side int) *painter {
	parts := layers(c, side)
	outline, glow := float64(c.Outline), float64(c.Glow)
	kept := make([][]field, len(parts))
	for index, part := range parts {
		kept[index] = make([]field, 0, len(part.fields))
	}
	return &painter{
		parts:        parts,
		near:         slices.Clone(parts),
		kept:         kept,
		distances:    make([]float64, len(parts)),
		glowColor:    colorOr(c.Color, [4]byte{}),
		outlineColor: colorOr(c.OutlineColor, [4]byte{}),
		outline:      outline,
		glow:         glow,
		reach:        max(glow, outline+0.5, 0.5),
	}
}

// measure returns the distance from a point to the nearest part.
func (p *painter) measure(x, y float64) float64 {
	nearest := math.Inf(1)
	for index, part := range p.near {
		p.distances[index] = part.distance(x, y)
		nearest = min(nearest, p.distances[index])
	}
	return nearest
}

// focus keeps the fields that could reach a tile, and reports whether there are any.
// A distance changes by at most one for each pixel moved, so a field further than
// the tile's width from its middle can't touch anything inside it.
func (p *painter) focus(left, top int) bool {
	x, y := float64(left+tileSide/2), float64(top+tileSide/2)
	limit := p.reach + tileSide
	p.near = p.near[:0]
	for index, part := range p.parts {
		if part.box != nil && !part.box.reaches(x, y, limit) {
			continue
		}
		kept := p.kept[index][:0]
		for _, each := range part.fields {
			if !(each(x, y) >= limit) {
				kept = append(kept, each)
			}
		}
		p.kept[index] = kept
		if len(kept) > 0 {
			p.near = append(p.near, layer{fields: kept, color: part.color})
		}
	}
	return len(p.near) > 0
}

// paint colours one pixel: glow first, then the outline, then each part.
func (p *painter) paint(x, y int, out []byte) {
	nearest := p.measure(float64(x)+0.5, float64(y)+0.5)
	if nearest >= p.reach {
		return
	}
	var pixel mixer
	if nearest > 0 && nearest < p.glow {
		fade := 1 - nearest/p.glow
		pixel.add(p.glowColor, 0.55*fade*fade)
	}
	if p.outline > 0 {
		pixel.add(p.outlineColor, coverage(nearest-p.outline))
	}
	for index, part := range p.near {
		pixel.add(part.color, coverage(p.distances[index]))
	}
	pixel.write(out)
}

// Render draws the crosshair and anything merged with it. Each merged crosshair is
// drawn on its own and then stacked, so it keeps its own outline and glow.
func Render(c Config) Image {
	side := windowSide(c)
	main := renderOne(c, side)
	if len(c.Merged) == 0 {
		return main
	}
	img := Image{Side: side, Pix: make([]byte, side*side*4)}
	for _, behind := range []bool{true, false} {
		if !behind {
			img.lay(main, 0, 0, 1)
		}
		for _, extra := range c.Merged {
			if extra.Behind == behind {
				img.lay(renderOne(extra, side), extra.OffsetX, extra.OffsetY, extra.Opacity)
			}
		}
	}
	return img
}

// lay places a picture of the same size on top of this one.
func (img Image) lay(top Image, moveX, moveY int, opacity float64) {
	side := img.Side
	for y := max(0, -moveY); y < min(side, side-moveY); y++ {
		for x := max(0, -moveX); x < min(side, side-moveX); x++ {
			from := top.Pix[(y*side+x)*4:]
			if from[3] == 0 {
				continue
			}
			to := img.Pix[((y+moveY)*side+x+moveX)*4:]
			pixel := mixer{[3]float64{float64(to[0]), float64(to[1]), float64(to[2])}, float64(to[3]) / 255}
			pixel.add([4]byte(from), float64(from[3])/255*opacity)
			pixel.write(to)
		}
	}
}

// renderOne draws one crosshair a tile at a time, because most of the window is empty.
func renderOne(c Config, side int) Image {
	img := Image{Side: side, Pix: make([]byte, side*side*4)}
	painter := newPainter(c, side)
	for top := 0; top < side; top += tileSide {
		for left := 0; left < side; left += tileSide {
			if !painter.focus(left, top) {
				continue
			}
			for y := top; y < min(top+tileSide, side); y++ {
				for x := left; x < min(left+tileSide, side); x++ {
					painter.paint(x, y, img.Pix[(y*side+x)*4:])
				}
			}
		}
	}
	if c.Image == "" {
		return img
	}
	picture := imageLayer(c, side)
	picture.lay(img, 0, 0, 1)
	return picture
}
