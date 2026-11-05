package main

// categories sets the order of the Crosshair Browser list in the sidebar. To add
// a category, add its name here and give some presets that Category.
var categories = []string{"Classic", "Dots", "Scopes", "Frames", "Minimal", "Bold", "High contrast"}

func shape(color string, length, gap, thickness, dot, outline int) Config {
	c := DefaultConfig()
	c.Color, c.Length, c.Gap, c.Thickness, c.DotSize, c.Outline = color, length, gap, thickness, dot, outline
	return c
}

func framed(c Config, frame string, size, thickness int) Config {
	c.Frame, c.FrameSize, c.FrameThickness = frame, size, thickness
	return c
}

func outlined(c Config, color string) Config {
	c.OutlineColor = color
	return c
}

func withHidden(c Config, arms string) Config {
	c.HideArms = arms
	return c
}

func withRotation(c Config, degrees int) Config {
	c.Rotation = degrees
	return c
}

func withRoundEnds(c Config) Config {
	c.RoundEnds = true
	return c
}

func withDotShape(c Config, name string) Config {
	c.DotShape = name
	return c
}

func withDotColor(c Config, color string) Config {
	c.DotColor = color
	return c
}

func withGlow(c Config, size int) Config {
	c.Glow = size
	return c
}

func withFrameGaps(c Config, gaps int) Config {
	c.FrameGaps = gaps
	return c
}

// presets are the built-in crosshairs shown in the Crosshair Browser.
var presets = []Crosshair{
	{ID: "classic-green", Name: "Classic green", Category: "Classic", Config: shape("#00FF00", 8, 4, 2, 0, 1)},
	{ID: "classic-cyan", Name: "Classic cyan", Category: "Classic", Config: shape("#00FFFF", 8, 4, 2, 0, 1)},
	{ID: "classic-white", Name: "Classic white", Category: "Classic", Config: shape("#FFFFFF", 8, 4, 2, 0, 1)},
	{ID: "classic-yellow", Name: "Classic yellow", Category: "Classic", Config: shape("#FFFF00", 8, 4, 2, 0, 1)},
	{ID: "tight-cross", Name: "Tight cross", Category: "Classic", Config: shape("#00FF00", 6, 2, 2, 0, 1)},
	{ID: "long-cross", Name: "Long cross", Category: "Classic", Config: withRoundEnds(shape("#FFFFFF", 18, 5, 2, 2, 1))},
	{ID: "compact-green", Name: "Compact green", Category: "Classic", Config: shape("#00FF00", 4, 2, 2, 0, 0)},
	{ID: "compact-cyan", Name: "Compact cyan", Category: "Classic", Config: shape("#00FFFF", 4, 2, 2, 0, 0)},
	{ID: "compact-white", Name: "Compact white", Category: "Classic", Config: shape("#FFFFFF", 4, 2, 2, 0, 0)},
	{ID: "compact-yellow", Name: "Compact yellow", Category: "Classic", Config: shape("#FFFF00", 4, 2, 2, 0, 0)},
	{ID: "compact-magenta", Name: "Compact magenta", Category: "Classic", Config: shape("#FF00FF", 4, 2, 2, 0, 0)},
	{ID: "short-gap-green", Name: "Short gap green", Category: "Classic", Config: shape("#00FF00", 3, 5, 2, 0, 0)},
	{ID: "short-gap-cyan", Name: "Short gap cyan", Category: "Classic", Config: shape("#00FFFF", 3, 5, 2, 0, 0)},
	{ID: "close-green", Name: "Close green", Category: "Classic", Config: shape("#00FF00", 5, 1, 2, 0, 1)},
	{ID: "close-cyan", Name: "Close cyan", Category: "Classic", Config: shape("#00FFFF", 5, 1, 2, 0, 1)},
	{ID: "close-white", Name: "Close white", Category: "Classic", Config: shape("#FFFFFF", 5, 1, 2, 0, 1)},
	{ID: "tall-white", Name: "Tall white", Category: "Classic", Config: shape("#FFFFFF", 7, 3, 2, 0, 1)},
	{ID: "tall-green", Name: "Tall green", Category: "Classic", Config: shape("#00FF00", 7, 3, 2, 0, 1)},
	{ID: "outlined-cyan", Name: "Outlined cyan", Category: "Classic", Config: shape("#00FFFF", 4, 3, 2, 0, 1)},
	{ID: "outlined-yellow", Name: "Outlined yellow", Category: "Classic", Config: shape("#FFFF00", 4, 3, 2, 0, 1)},
	{ID: "outlined-pink", Name: "Outlined pink", Category: "Classic", Config: shape("#FF2D95", 4, 3, 2, 0, 1)},
	{ID: "stubby-white", Name: "Stubby white", Category: "Classic", Config: shape("#FFFFFF", 2, 3, 3, 0, 0)},
	{ID: "stubby-green", Name: "Stubby green", Category: "Classic", Config: shape("#00FF00", 2, 3, 3, 0, 0)},
	{ID: "rounded-green", Name: "Rounded green", Category: "Classic", Config: withRoundEnds(shape("#00FF00", 6, 3, 2, 0, 1))},
	{ID: "rounded-cyan", Name: "Rounded cyan", Category: "Classic", Config: withRoundEnds(shape("#00FFFF", 6, 3, 2, 0, 1))},
	{ID: "lime-cross", Name: "Lime cross", Category: "Classic", Config: shape("#B6FF00", 6, 3, 2, 0, 1)},

	{ID: "small-dot", Name: "Small dot", Category: "Dots", Config: shape("#00FF00", 0, 0, 2, 2, 1)},
	{ID: "medium-dot", Name: "Medium dot", Category: "Dots", Config: shape("#FF00FF", 0, 0, 2, 4, 1)},
	{ID: "large-dot", Name: "Large dot", Category: "Dots", Config: shape("#FFFFFF", 0, 0, 2, 6, 1)},
	{ID: "red-dot", Name: "Red dot", Category: "Dots", Config: shape("#FF2020", 0, 0, 2, 4, 1)},
	{ID: "cross-and-dot", Name: "Cross and dot", Category: "Dots", Config: shape("#00FFFF", 6, 4, 2, 2, 1)},
	{ID: "ring-dot", Name: "Ring and dot", Category: "Dots", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "circle", 16, 3)},
	{ID: "tiny-cyan-dot", Name: "Tiny cyan dot", Category: "Dots", Config: shape("#00FFFF", 0, 0, 2, 2, 1)},
	{ID: "tiny-yellow-dot", Name: "Tiny yellow dot", Category: "Dots", Config: shape("#FFFF00", 0, 0, 2, 2, 1)},
	{ID: "tiny-white-dot", Name: "Tiny white dot", Category: "Dots", Config: shape("#FFFFFF", 0, 0, 2, 2, 1)},
	{ID: "round-green-dot", Name: "Round green dot", Category: "Dots", Config: withDotShape(shape("#00FF00", 0, 0, 2, 4, 1), "round")},
	{ID: "round-cyan-dot", Name: "Round cyan dot", Category: "Dots", Config: withDotShape(shape("#00FFFF", 0, 0, 2, 4, 1), "round")},
	{ID: "round-red-dot", Name: "Round red dot", Category: "Dots", Config: withDotShape(shape("#FF2020", 0, 0, 2, 4, 1), "round")},
	{ID: "round-white-dot", Name: "Round white dot", Category: "Dots", Config: withDotShape(shape("#FFFFFF", 0, 0, 2, 6, 1), "round")},
	{ID: "diamond-green-dot", Name: "Diamond green dot", Category: "Dots", Config: withDotShape(shape("#00FF00", 0, 0, 2, 4, 1), "diamond")},
	{ID: "diamond-cyan-dot", Name: "Diamond cyan dot", Category: "Dots", Config: withDotShape(shape("#00FFFF", 0, 0, 2, 6, 1), "diamond")},
	{ID: "diamond-yellow-dot", Name: "Diamond yellow dot", Category: "Dots", Config: withDotShape(shape("#FFFF00", 0, 0, 2, 4, 1), "diamond")},
	{ID: "pink-pixel-dot", Name: "Pink pixel dot", Category: "Dots", Config: shape("#FF2D95", 0, 0, 2, 3, 1)},
	{ID: "dot-and-ticks", Name: "Dot and ticks", Category: "Dots", Config: shape("#00FF00", 2, 4, 2, 2, 0)},
	{ID: "cyan-dot-and-ticks", Name: "Cyan dot and ticks", Category: "Dots", Config: shape("#00FFFF", 2, 4, 2, 2, 0)},
	{ID: "red-dot-white-ticks", Name: "Red dot, white ticks", Category: "Dots", Config: withDotColor(shape("#FFFFFF", 3, 4, 2, 2, 0), "#FF2020")},
	{ID: "tiny-ring", Name: "Tiny ring", Category: "Dots", Config: framed(shape("#00FFFF", 0, 0, 2, 2, 1), "circle", 8, 1)},

	{ID: "scope-dot", Name: "Scope dot", Category: "Scopes", Config: framed(withDotShape(shape("#00FF00", 10, 13, 3, 3, 1), "round"), "circle", 18, 3)},
	{ID: "red-dot-scope", Name: "Red dot scope", Category: "Scopes", Config: withDotColor(framed(withDotShape(shape("#FFFFFF", 10, 13, 3, 7, 1), "round"), "circle", 18, 3), "#FF2020")},
	{ID: "broken-scope", Name: "Broken scope", Category: "Scopes", Config: withFrameGaps(framed(shape("#00FFFF", 9, 8, 2, 2, 1), "circle", 16, 2), 4)},
	{ID: "open-scope", Name: "Open scope", Category: "Scopes", Config: framed(withRoundEnds(shape("#FFFFFF", 12, 14, 3, 0, 1)), "circle", 20, 3)},
	{ID: "field-scope", Name: "Field scope", Category: "Scopes", Config: framed(shape("#00FF00", 10, 10, 1, 1, 1), "circle", 14, 1)},
	{ID: "long-range-scope", Name: "Long range scope", Category: "Scopes", Config: framed(shape("#FFFFFF", 16, 12, 1, 1, 1), "circle", 20, 1)},
	{ID: "cyan-sight", Name: "Cyan sight", Category: "Scopes", Config: framed(shape("#00FFFF", 8, 10, 2, 2, 1), "circle", 14, 2)},
	{ID: "yellow-sight", Name: "Yellow sight", Category: "Scopes", Config: framed(shape("#FFFF00", 8, 10, 2, 2, 1), "circle", 14, 2)},
	{ID: "red-ring-sight", Name: "Red ring sight", Category: "Scopes", Config: withDotColor(framed(shape("#FFFFFF", 6, 12, 1, 2, 1), "circle", 16, 2), "#FF2020")},
	{ID: "rangefinder", Name: "Rangefinder", Category: "Scopes", Config: withHidden(framed(shape("#FFFFFF", 14, 6, 1, 2, 1), "circle", 18, 1), "t")},
	{ID: "open-top-scope", Name: "Open top scope", Category: "Scopes", Config: withHidden(framed(shape("#00FF00", 12, 8, 2, 2, 1), "circle", 16, 1), "t")},
	{ID: "split-scope", Name: "Split scope", Category: "Scopes", Config: withFrameGaps(framed(shape("#00FFFF", 0, 0, 2, 2, 1), "circle", 16, 2), 2)},
	{ID: "three-gap-scope", Name: "Three gap scope", Category: "Scopes", Config: withFrameGaps(framed(shape("#00FF00", 0, 0, 2, 2, 1), "circle", 16, 2), 3)},
	{ID: "six-gap-scope", Name: "Six gap scope", Category: "Scopes", Config: withFrameGaps(framed(shape("#FFFFFF", 0, 0, 2, 2, 1), "circle", 18, 2), 6)},
	{ID: "eight-gap-scope", Name: "Eight gap scope", Category: "Scopes", Config: withFrameGaps(framed(shape("#FFFF00", 0, 0, 2, 2, 1), "circle", 18, 2), 8)},
	{ID: "tilted-scope", Name: "Tilted scope", Category: "Scopes", Config: withRotation(framed(shape("#00FF00", 10, 10, 2, 0, 1), "circle", 14, 1), 45)},
	{ID: "crossed-ring", Name: "Crossed ring", Category: "Scopes", Config: framed(shape("#FFFFFF", 24, 0, 1, 0, 1), "circle", 14, 1)},
	{ID: "close-quarters-scope", Name: "Close quarters scope", Category: "Scopes", Config: framed(shape("#FF2020", 4, 6, 2, 2, 1), "circle", 10, 2)},

	{ID: "focus-frame", Name: "Focus frame", Category: "Frames", Config: framed(withRoundEnds(shape("#FFFFFF", 4, 0, 2, 0, 1)), "brackets", 18, 3)},
	{ID: "corner-marks", Name: "Corner marks", Category: "Frames", Config: withRotation(withRoundEnds(shape("#00FFFF", 7, 16, 4, 3, 1)), 45)},
	{ID: "square-frame", Name: "Square frame", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "square", 14, 2)},
	{ID: "hex-frame", Name: "Hex frame", Category: "Frames", Config: framed(shape("#FFFF00", 0, 0, 2, 2, 1), "hexagon", 18, 2)},
	{ID: "small-ring", Name: "Small ring", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 0, 1), "circle", 9, 2)},
	{ID: "wide-ring", Name: "Wide ring", Category: "Frames", Config: framed(shape("#FFFFFF", 0, 0, 2, 2, 1), "circle", 22, 1)},
	{ID: "cyan-box", Name: "Cyan box", Category: "Frames", Config: framed(shape("#00FFFF", 0, 0, 2, 2, 1), "square", 12, 2)},
	{ID: "wide-box", Name: "Wide box", Category: "Frames", Config: framed(shape("#FFFFFF", 0, 0, 2, 2, 1), "square", 20, 2)},
	{ID: "green-diamond", Name: "Green diamond", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "diamond", 14, 2)},
	{ID: "yellow-diamond", Name: "Yellow diamond", Category: "Frames", Config: framed(shape("#FFFF00", 0, 0, 2, 0, 1), "diamond", 18, 2)},
	{ID: "green-triangle", Name: "Green triangle", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "triangle", 16, 2)},
	{ID: "cyan-triangle", Name: "Cyan triangle", Category: "Frames", Config: framed(shape("#00FFFF", 0, 0, 2, 0, 1), "triangle", 20, 2)},
	{ID: "pink-hexagon", Name: "Pink hexagon", Category: "Frames", Config: framed(shape("#FF2D95", 0, 0, 2, 2, 1), "hexagon", 14, 2)},
	{ID: "turned-hexagon", Name: "Turned hexagon", Category: "Frames", Config: withRotation(framed(shape("#00FFFF", 0, 0, 2, 2, 1), "hexagon", 16, 2), 30)},
	{ID: "green-chevron", Name: "Green chevron", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "chevron", 12, 2)},
	{ID: "white-chevron", Name: "White chevron", Category: "Frames", Config: framed(shape("#FFFFFF", 0, 0, 2, 0, 1), "chevron", 16, 2)},
	{ID: "green-brackets", Name: "Green brackets", Category: "Frames", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "brackets", 12, 2)},
	{ID: "white-brackets", Name: "White brackets", Category: "Frames", Config: framed(shape("#FFFFFF", 0, 0, 2, 2, 1), "brackets", 16, 2)},
	{ID: "bracketed-cross", Name: "Bracketed cross", Category: "Frames", Config: framed(shape("#00FFFF", 3, 3, 2, 0, 0), "brackets", 14, 2)},
	{ID: "boxed-cross", Name: "Boxed cross", Category: "Frames", Config: framed(shape("#00FF00", 4, 3, 2, 0, 0), "square", 14, 2)},

	{ID: "hairline-white", Name: "Hairline white", Category: "Minimal", Config: shape("#FFFFFF", 8, 4, 1, 0, 0)},
	{ID: "hairline-green", Name: "Hairline green", Category: "Minimal", Config: shape("#00FF00", 8, 4, 1, 0, 0)},
	{ID: "fine-cross", Name: "Fine cross", Category: "Minimal", Config: shape("#00FF00", 5, 3, 1, 0, 1)},
	{ID: "pin-dot", Name: "Pin dot", Category: "Minimal", Config: shape("#FFFFFF", 0, 0, 1, 1, 1)},
	{ID: "hairline-cyan", Name: "Hairline cyan", Category: "Minimal", Config: shape("#00FFFF", 8, 4, 1, 0, 0)},
	{ID: "hairline-yellow", Name: "Hairline yellow", Category: "Minimal", Config: shape("#FFFF00", 8, 4, 1, 0, 0)},
	{ID: "hairline-pink", Name: "Hairline pink", Category: "Minimal", Config: shape("#FF2D95", 8, 4, 1, 0, 0)},
	{ID: "micro-green", Name: "Micro green", Category: "Minimal", Config: shape("#00FF00", 3, 2, 1, 0, 0)},
	{ID: "micro-cyan", Name: "Micro cyan", Category: "Minimal", Config: shape("#00FFFF", 3, 2, 1, 0, 0)},
	{ID: "micro-white", Name: "Micro white", Category: "Minimal", Config: shape("#FFFFFF", 3, 2, 1, 0, 0)},
	{ID: "micro-x", Name: "Micro X", Category: "Minimal", Config: withRotation(shape("#FFFFFF", 4, 3, 1, 0, 0), 45)},
	{ID: "t-shape", Name: "T shape", Category: "Minimal", Config: withHidden(shape("#00FF00", 6, 3, 1, 0, 0), "t")},
	{ID: "t-shape-cyan", Name: "T shape cyan", Category: "Minimal", Config: withHidden(shape("#00FFFF", 6, 3, 1, 0, 0), "t")},
	{ID: "side-ticks", Name: "Side ticks", Category: "Minimal", Config: withHidden(shape("#FFFFFF", 6, 4, 2, 0, 0), "tb")},
	{ID: "bottom-tick", Name: "Bottom tick", Category: "Minimal", Config: withHidden(shape("#00FF00", 8, 3, 2, 0, 0), "trl")},
	{ID: "open-v", Name: "Open V", Category: "Minimal", Config: withRotation(withHidden(shape("#00FFFF", 8, 3, 2, 0, 0), "rb"), 45)},
	{ID: "pin-dot-cyan", Name: "Pin dot cyan", Category: "Minimal", Config: shape("#00FFFF", 0, 0, 1, 1, 1)},
	{ID: "pin-dot-yellow", Name: "Pin dot yellow", Category: "Minimal", Config: shape("#FFFF00", 0, 0, 1, 1, 1)},
	{ID: "ghost-dot-ring", Name: "Ghost dot ring", Category: "Minimal", Config: framed(shape("#FFFFFF", 0, 0, 1, 1, 0), "circle", 12, 1)},

	{ID: "heavy-cross", Name: "Heavy cross", Category: "Bold", Config: shape("#00FF00", 12, 5, 4, 0, 2)},
	{ID: "wide-gap", Name: "Wide gap", Category: "Bold", Config: shape("#FFFF00", 10, 10, 3, 0, 1)},
	{ID: "solid-plus", Name: "Solid plus", Category: "Bold", Config: shape("#00FFFF", 10, 0, 4, 0, 1)},
	{ID: "big-block", Name: "Big block", Category: "Bold", Config: shape("#FFFFFF", 14, 6, 6, 0, 2)},
	{ID: "heavy-ring", Name: "Heavy ring", Category: "Bold", Config: framed(shape("#FFFF00", 0, 0, 2, 0, 1), "circle", 14, 6)},
	{ID: "heavy-cyan", Name: "Heavy cyan", Category: "Bold", Config: shape("#00FFFF", 12, 5, 4, 0, 2)},
	{ID: "heavy-white", Name: "Heavy white", Category: "Bold", Config: shape("#FFFFFF", 12, 5, 4, 0, 2)},
	{ID: "thick-plus", Name: "Thick plus", Category: "Bold", Config: shape("#00FF00", 12, 0, 6, 0, 1)},
	{ID: "block-x", Name: "Block X", Category: "Bold", Config: withRotation(shape("#FFFF00", 10, 4, 4, 0, 1), 45)},
	{ID: "wide-bars", Name: "Wide bars", Category: "Bold", Config: shape("#FF2020", 14, 8, 5, 0, 2)},
	{ID: "rounded-heavy", Name: "Rounded heavy", Category: "Bold", Config: withRoundEnds(shape("#00FF00", 10, 5, 5, 0, 1))},
	{ID: "big-dot-cross", Name: "Big dot cross", Category: "Bold", Config: shape("#FFFFFF", 10, 8, 3, 6, 1)},
	{ID: "heavy-box", Name: "Heavy box", Category: "Bold", Config: framed(shape("#00FFFF", 0, 0, 2, 0, 1), "square", 14, 4)},
	{ID: "heavy-triangle", Name: "Heavy triangle", Category: "Bold", Config: framed(shape("#FFFF00", 0, 0, 2, 0, 1), "triangle", 18, 4)},
	{ID: "thick-brackets", Name: "Thick brackets", Category: "Bold", Config: framed(shape("#00FF00", 0, 0, 2, 0, 1), "brackets", 16, 4)},

	{ID: "hot-pink", Name: "Hot pink", Category: "High contrast", Config: shape("#FF2D95", 9, 4, 3, 0, 2)},
	{ID: "signal-orange", Name: "Signal orange", Category: "High contrast", Config: shape("#FF8000", 9, 4, 3, 0, 2)},
	{ID: "black-on-white", Name: "Black on white", Category: "High contrast", Config: outlined(shape("#000000", 9, 4, 2, 0, 2), "#FFFFFF")},
	{ID: "white-with-black-edge", Name: "White with black edge", Category: "High contrast", Config: shape("#FFFFFF", 8, 4, 2, 0, 2)},
	{ID: "yellow-with-black-edge", Name: "Yellow with black edge", Category: "High contrast", Config: shape("#FFFF00", 8, 4, 3, 0, 2)},
	{ID: "cyan-with-black-edge", Name: "Cyan with black edge", Category: "High contrast", Config: shape("#00FFFF", 8, 4, 3, 0, 2)},
	{ID: "magenta-with-white-edge", Name: "Magenta with white edge", Category: "High contrast", Config: outlined(shape("#FF00FF", 8, 4, 2, 0, 2), "#FFFFFF")},
	{ID: "black-with-yellow-edge", Name: "Black with yellow edge", Category: "High contrast", Config: outlined(shape("#000000", 8, 4, 2, 0, 2), "#FFFF00")},
	{ID: "lime-with-black-edge", Name: "Lime with black edge", Category: "High contrast", Config: shape("#B6FF00", 8, 4, 3, 0, 2)},
	{ID: "neon-green", Name: "Neon green", Category: "High contrast", Config: withGlow(shape("#00FF00", 8, 4, 2, 0, 0), 6)},
	{ID: "neon-cyan", Name: "Neon cyan", Category: "High contrast", Config: withGlow(shape("#00FFFF", 8, 4, 2, 0, 0), 6)},
	{ID: "neon-pink", Name: "Neon pink", Category: "High contrast", Config: withGlow(shape("#FF2D95", 8, 4, 2, 0, 0), 6)},
	{ID: "neon-dot", Name: "Neon dot", Category: "High contrast", Config: withGlow(withDotShape(shape("#00E5C0", 0, 0, 2, 4, 0), "round"), 8)},
}

// baseGroups and bases are the starting points in the Crosshair Editor.
var baseGroups = []string{"Lines", "Dots and circles", "Shapes"}

var bases = []Crosshair{
	{ID: "base-cross", Name: "Cross", Category: "Lines", Config: shape("#00FF00", 8, 4, 2, 0, 1)},
	{ID: "base-cross-dot", Name: "Cross and dot", Category: "Lines", Config: shape("#00FF00", 8, 5, 2, 2, 1)},
	{ID: "base-plus", Name: "Solid plus", Category: "Lines", Config: shape("#00FF00", 10, 0, 2, 0, 1)},
	{ID: "base-x", Name: "X", Category: "Lines", Config: withRotation(shape("#00FF00", 8, 4, 2, 0, 1), 45)},
	{ID: "base-t", Name: "T shape", Category: "Lines", Config: withHidden(shape("#00FF00", 8, 4, 2, 0, 1), "t")},
	{ID: "base-line", Name: "Line", Category: "Lines", Config: withHidden(shape("#00FF00", 12, 4, 2, 0, 1), "tb")},
	{ID: "base-round-cross", Name: "Round cross", Category: "Lines", Config: withRoundEnds(shape("#00FF00", 8, 4, 4, 0, 1))},

	{ID: "base-dot", Name: "Dot", Category: "Dots and circles", Config: shape("#00FF00", 0, 0, 2, 4, 1)},
	{ID: "base-round-dot", Name: "Round dot", Category: "Dots and circles", Config: withDotShape(shape("#00FF00", 0, 0, 2, 8, 1), "round")},
	{ID: "base-diamond-dot", Name: "Diamond dot", Category: "Dots and circles", Config: withDotShape(shape("#00FF00", 0, 0, 2, 8, 1), "diamond")},
	{ID: "base-circle", Name: "Circle", Category: "Dots and circles", Config: framed(shape("#00FF00", 0, 0, 2, 0, 1), "circle", 16, 2)},
	{ID: "base-circle-dot", Name: "Circle and dot", Category: "Dots and circles", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "circle", 16, 2)},
	{ID: "base-broken-circle", Name: "Broken circle", Category: "Dots and circles", Config: withFrameGaps(framed(shape("#00FF00", 0, 0, 2, 2, 1), "circle", 18, 2), 4)},
	{ID: "base-scope", Name: "Scope", Category: "Dots and circles", Config: framed(shape("#00FF00", 12, 12, 2, 0, 1), "circle", 18, 2)},

	{ID: "base-square", Name: "Square", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "square", 14, 2)},
	{ID: "base-diamond", Name: "Diamond", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "diamond", 18, 2)},
	{ID: "base-triangle", Name: "Triangle", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "triangle", 18, 2)},
	{ID: "base-hexagon", Name: "Hexagon", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "hexagon", 18, 2)},
	{ID: "base-chevron", Name: "Chevron", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 0, 1), "chevron", 14, 3)},
	{ID: "base-brackets", Name: "Brackets", Category: "Shapes", Config: framed(shape("#00FF00", 0, 0, 2, 2, 1), "brackets", 16, 2)},
}
