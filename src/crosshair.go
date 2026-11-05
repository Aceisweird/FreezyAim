// Settings for Freezy Aim. This file works the same on every system.
//
// To add a setting: add a field to Config, then use it in layers() in shapes.go.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	configName = "crosshair_config.json"
	byteOrder  = "\xef\xbb\xbf" // Some Windows editors start files with this.

	// maxPixels caps size settings so a typo can't allocate a huge window.
	maxPixels = 512

	maxGlow      = 32
	maxFrameGaps = 8

	maxMerged       = 3    // How many crosshairs can be merged into one.
	maxStrokes      = 300  // How many lines one drawing can have.
	maxStrokePoints = 4000 // How many points all of those lines can have together.
	maxStrokeWidth  = 40
)

var (
	frameNames = []string{"circle", "square", "diamond", "triangle", "hexagon", "chevron", "brackets"}
	dotShapes  = []string{"round", "diamond"}
)

// configLock stops the overlay reading the file while the app window replaces it.
var configLock sync.Mutex

// Config matches crosshair_config.json.
type Config struct {
	Color        string  `json:"color"`
	Opacity      float64 `json:"opacity"`
	Length       int     `json:"length"`    // Length of each arm, in pixels.
	Gap          int     `json:"gap"`       // Space between the centre and each arm.
	Thickness    int     `json:"thickness"` // Width of each arm.
	DotSize      int     `json:"dot_size"`  // Centre dot size. 0 turns it off.
	Outline      int     `json:"outline"`   // Outline width. 0 turns it off.
	OutlineColor string  `json:"outline_color"`
	OffsetX      int     `json:"offset_x"` // Nudge from screen centre. Positive moves right.
	OffsetY      int     `json:"offset_y"` // Nudge from screen centre. Positive moves down.
	Screen       int     `json:"screen"`   // Monitor number. 0 is the primary monitor.

	// The settings below are left out of the file when unused.
	HideArms       string `json:"hide_arms,omitempty"`       // Arms to hide: any of t, r, b and l.
	Rotation       int    `json:"rotation,omitempty"`        // Degrees clockwise. 45 turns a cross into an X.
	RoundEnds      bool   `json:"round_ends,omitempty"`      // Rounds the ends of arms and frame lines.
	DotShape       string `json:"dot_shape,omitempty"`       // "round" or "diamond". Empty is square.
	DotColor       string `json:"dot_color,omitempty"`       // Empty matches Color.
	Frame          string `json:"frame,omitempty"`           // A shape around the centre, from frameNames.
	FrameSize      int    `json:"frame_size,omitempty"`      // Distance from the centre to the frame.
	FrameThickness int    `json:"frame_thickness,omitempty"` // Width of the frame line.
	FrameGaps      int    `json:"frame_gaps,omitempty"`      // Openings cut in the frame.
	FrameColor     string `json:"frame_color,omitempty"`     // Empty matches Color.
	Glow           int    `json:"glow,omitempty"`            // Soft glow radius. 0 turns it off.

	Strokes []Stroke `json:"strokes,omitempty"` // Lines from the Draw page.

	Image     string `json:"picture,omitempty"`      // A picture under everything else: base64 PNG.
	ImageSize int    `json:"picture_size,omitempty"` // Width or height of the picture, whichever is larger.
	ImageX    int    `json:"picture_x,omitempty"`    // Moves the picture. Positive moves right.
	ImageY    int    `json:"picture_y,omitempty"`    // Moves the picture. Positive moves down.

	// Merged crosshairs are stacked with this one and keep their own look. Their
	// offsets move them relative to it, and their opacity fades only them.
	Merged []Config `json:"merged,omitempty"`
	Behind bool     `json:"behind,omitempty"` // A merged crosshair sits under this one.
}

// Stroke is one line drawn by hand. Points are x, y pairs, in pixels from the centre.
type Stroke struct {
	Color  string    `json:"color"`
	Width  float64   `json:"width"`
	Points []float64 `json:"points"`
	Fill   bool      `json:"fill,omitempty"` // Fills the shape the line goes around.
}

func DefaultConfig() Config {
	return Config{
		Color:        "#00FF00",
		Opacity:      1,
		Length:       8,
		Gap:          4,
		Thickness:    2,
		Outline:      1,
		OutlineColor: "#000000",
	}
}

// ParseConfig reads settings. Missing keys keep their defaults, so older
// config files carry on working when new settings are added.
func ParseConfig(data []byte) (Config, error) {
	c := DefaultConfig()
	data = bytes.TrimPrefix(data, []byte(byteOrder))
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if err := c.clean(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// clean keeps numbers in range, and reports anything that can't be drawn.
func (c *Config) clean() error {
	for _, size := range []*int{&c.Length, &c.Gap, &c.Thickness, &c.DotSize, &c.Outline, &c.FrameSize, &c.FrameThickness} {
		*size = min(max(*size, 0), maxPixels)
	}
	c.Opacity = min(max(c.Opacity, 0), 1)
	for _, color := range []string{c.Color, c.OutlineColor} {
		if _, err := parseColor(color); err != nil {
			return err
		}
	}
	for _, color := range []string{c.DotColor, c.FrameColor} {
		if _, err := parseColor(color); color != "" && err != nil {
			return err
		}
	}
	if c.Frame != "" && !slices.Contains(frameNames, c.Frame) {
		return fmt.Errorf("%q is not a frame style", c.Frame)
	}
	if c.DotShape != "" && !slices.Contains(dotShapes, c.DotShape) {
		return fmt.Errorf("%q is not a dot shape", c.DotShape)
	}
	if strings.Trim(c.HideArms, "trbl") != "" {
		return fmt.Errorf("%q isn't a list of arms", c.HideArms)
	}
	c.Rotation = (c.Rotation%360 + 360) % 360
	c.FrameGaps = min(max(c.FrameGaps, 0), maxFrameGaps)
	c.Glow = min(max(c.Glow, 0), maxGlow)
	if err := checkStrokes(c.Strokes); err != nil {
		return err
	}
	if err := cleanImage(c); err != nil {
		return err
	}
	if len(c.Merged) > maxMerged {
		return fmt.Errorf("A crosshair can only merge with %d others.", maxMerged)
	}
	for index := range c.Merged {
		extra := &c.Merged[index]
		extra.Merged = nil // Merging is one level deep.
		extra.OffsetX = min(max(extra.OffsetX, -maxPixels), maxPixels)
		extra.OffsetY = min(max(extra.OffsetY, -maxPixels), maxPixels)
		if err := extra.clean(); err != nil {
			return err
		}
	}
	return nil
}

func checkStrokes(strokes []Stroke) error {
	if len(strokes) > maxStrokes {
		return errors.New("That drawing has too many lines. Erase some, then try again.")
	}
	points := 0
	for index := range strokes {
		stroke := &strokes[index]
		if _, err := parseColor(stroke.Color); err != nil {
			return err
		}
		if len(stroke.Points) < 2 || len(stroke.Points)%2 != 0 {
			return errors.New("That drawing has a broken line.")
		}
		for _, value := range stroke.Points {
			if value < -maxPixels || value > maxPixels {
				return errors.New("That drawing goes too far from the center.")
			}
		}
		stroke.Width = min(max(stroke.Width, 1), maxStrokeWidth)
		points += len(stroke.Points) / 2
	}
	if points > maxStrokePoints {
		return errors.New("That drawing is too detailed. Erase some lines, then try again.")
	}
	return nil
}

// LoadConfig reads the settings file, creating it first if it doesn't exist.
func LoadConfig(path string) (Config, error) {
	configLock.Lock()
	defer configLock.Unlock()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = json.MarshalIndent(DefaultConfig(), "", "  ")
		if err != nil {
			return Config{}, err
		}
		if err := writeFileSafely(path, append(data, '\n')); err != nil {
			return Config{}, err
		}
	} else if err != nil {
		return Config{}, err
	}
	return ParseConfig(data)
}

// SaveConfig replaces the settings file.
func SaveConfig(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	configLock.Lock()
	defer configLock.Unlock()
	return writeFileSafely(path, append(data, '\n'))
}

// writeFileSafely swaps the new file into place so a reader never sees half of
// it. If another program is holding the file, it writes over it instead.
func writeFileSafely(path string, data []byte) error {
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		os.Remove(temp)
		return os.WriteFile(path, data, 0o644)
	}
	return nil
}

// parseColor turns "#RRGGBB" into blue, green, red and alpha bytes, the order
// Windows expects them in.
func parseColor(text string) ([4]byte, error) {
	if len(text) == 7 && text[0] == '#' {
		if value, err := strconv.ParseUint(text[1:], 16, 32); err == nil {
			return [4]byte{byte(value), byte(value >> 8), byte(value >> 16), 255}, nil
		}
	}
	return [4]byte{}, fmt.Errorf("%q is not a #RRGGBB colour", text)
}

// Image is a square picture stored as top-to-bottom BGRA pixels.
type Image struct {
	Side int
	Pix  []byte
}

// Premultiplied returns the form Windows needs for see-through windows.
func (img Image) Premultiplied() Image {
	out := Image{Side: img.Side, Pix: bytes.Clone(img.Pix)}
	for i := 0; i < len(out.Pix); i += 4 {
		alpha := int(out.Pix[i+3])
		for channel := 0; channel < 3; channel++ {
			out.Pix[i+channel] = byte(int(out.Pix[i+channel]) * alpha / 255)
		}
	}
	return out
}
