// Pictures imported as crosshairs. The browser shrinks them and turns them into PNG, so
// a picture travels with its crosshair: in saved files, previews and share codes.
package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"math"
	"sync"
)

const (
	maxImageData   = 200_000 // Base64 characters, which is about 150 KB.
	maxImagePixels = 512     // The widest picture that can be stored.
	maxImageSize   = 200     // The widest it can be shown, in screen pixels.
	minImageSize   = 4
	maxImageMove   = 200
	defaultImage   = 64
	maxSamples     = 6 // Samples per side of a pixel when shrinking.
	maxKeptImages  = 32
	solidAlpha     = 128 // Pixels this solid count as the picture when outlining it.
)

var (
	imageCache = map[string]*image.NRGBA{}
	imageLock  sync.Mutex
)

func cleanImage(c *Config) error {
	if c.Image == "" {
		c.ImageSize, c.ImageX, c.ImageY = 0, 0, 0
		return nil
	}
	if len(c.Image) > maxImageData {
		return errors.New("That picture is too large.")
	}
	data, err := base64.StdEncoding.DecodeString(c.Image)
	if err != nil {
		return errors.New("That picture is damaged.")
	}
	info, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width < 1 || info.Height < 1 || info.Width > maxImagePixels || info.Height > maxImagePixels {
		return errors.New("That picture can't be used.")
	}
	if c.ImageSize == 0 {
		c.ImageSize = defaultImage
	}
	c.ImageSize = min(max(c.ImageSize, minImageSize), maxImageSize)
	c.ImageX = min(max(c.ImageX, -maxImageMove), maxImageMove)
	c.ImageY = min(max(c.ImageY, -maxImageMove), maxImageMove)
	return nil
}

// imageReach is how far a picture can reach from the centre, however it's shaped.
func imageReach(c Config) int {
	if c.Image == "" {
		return 0
	}
	return (c.ImageSize+1)/2 + max(abs(c.ImageX), abs(c.ImageY)) + 1
}

// loadImage decodes a picture once, because previews ask for the same few repeatedly.
func loadImage(text string) *image.NRGBA {
	imageLock.Lock()
	defer imageLock.Unlock()
	if picture, ok := imageCache[text]; ok {
		return picture
	}
	data, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	picture := image.NewNRGBA(decoded.Bounds())
	draw.Draw(picture, picture.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	if len(imageCache) >= maxKeptImages {
		imageCache = map[string]*image.NRGBA{}
	}
	imageCache[text] = picture
	return picture
}

// sampleImage reads a blend of the four nearest pixels, weighted by how solid they are.
func sampleImage(picture *image.NRGBA, x, y float64) (red, green, blue, alpha float64) {
	bounds := picture.Bounds()
	x, y = x-0.5, y-0.5
	left, top := math.Floor(x), math.Floor(y)
	shareX, shareY := x-left, y-top
	for _, spot := range [4][3]float64{
		{left, top, (1 - shareX) * (1 - shareY)},
		{left + 1, top, shareX * (1 - shareY)},
		{left, top + 1, (1 - shareX) * shareY},
		{left + 1, top + 1, shareX * shareY},
	} {
		column := min(max(int(spot[0]), 0), bounds.Dx()-1)
		row := min(max(int(spot[1]), 0), bounds.Dy()-1)
		pixel := picture.Pix[row*picture.Stride+column*4:]
		solid := float64(pixel[3]) / 255 * spot[2]
		red += float64(pixel[0]) * solid
		green += float64(pixel[1]) * solid
		blue += float64(pixel[2]) * solid
		alpha += solid
	}
	return red, green, blue, alpha
}

// imageLayer draws a crosshair's picture into a window of the given size. Everything
// turns around the crosshair's centre, so a moved picture swings around it too.
func imageLayer(c Config, side int) Image {
	img := Image{Side: side, Pix: make([]byte, side*side*4)}
	picture := loadImage(c.Image)
	if picture == nil {
		return img
	}
	sourceWidth, sourceHeight := picture.Bounds().Dx(), picture.Bounds().Dy()
	shrink := float64(c.ImageSize) / float64(max(sourceWidth, sourceHeight))
	width, height := float64(sourceWidth)*shrink, float64(sourceHeight)*shrink
	samples := min(max(int(math.Ceil(1/shrink)), 1), maxSamples)
	left := float64(side)/2 + float64(c.ImageX) - width/2
	top := float64(side)/2 + float64(c.ImageY) - height/2
	center := float64(side) / 2
	sine, cosine := math.Sincos(float64(c.Rotation) * math.Pi / 180)
	each := 1 / float64(samples*samples)
	for y := range side {
		for x := range side {
			var red, green, blue, alpha float64
			for down := range samples {
				for along := range samples {
					spotX := float64(x) + (float64(along)+0.5)/float64(samples) - center
					spotY := float64(y) + (float64(down)+0.5)/float64(samples) - center
					fromX := (spotX*cosine + spotY*sine + center - left) / shrink
					fromY := (spotY*cosine - spotX*sine + center - top) / shrink
					if fromX < 0 || fromY < 0 || fromX >= float64(sourceWidth) || fromY >= float64(sourceHeight) {
						continue
					}
					r, g, b, a := sampleImage(picture, fromX, fromY)
					red, green, blue, alpha = red+r, green+g, blue+b, alpha+a
				}
			}
			if alpha == 0 {
				continue
			}
			to := img.Pix[(y*side+x)*4:]
			to[0], to[1], to[2] = byte(blue/alpha+0.5), byte(green/alpha+0.5), byte(red/alpha+0.5)
			to[3] = byte(alpha*each*255 + 0.5)
		}
	}
	if c.Outline > 0 || c.Glow > 0 {
		img.addHalo(c)
	}
	return img
}

// addHalo gives a picture the same outline and glow as any other crosshair. It finds how
// far each pixel is from the picture by sweeping the window forward and then back.
func (img Image) addHalo(c Config) {
	side := img.Side
	gaps := make([]float64, side*side)
	for index := range gaps {
		if img.Pix[index*4+3] < solidAlpha {
			gaps[index] = math.Inf(1)
		}
	}
	relax := func(x, y, stepX, stepY int) {
		for _, step := range [4][3]float64{{float64(stepX), 0, 1}, {0, float64(stepY), 1}, {float64(stepX), float64(stepY), math.Sqrt2}, {float64(-stepX), float64(stepY), math.Sqrt2}} {
			nearX, nearY := x+int(step[0]), y+int(step[1])
			if nearX >= 0 && nearY >= 0 && nearX < side && nearY < side {
				gaps[y*side+x] = min(gaps[y*side+x], gaps[nearY*side+nearX]+step[2])
			}
		}
	}
	for y := range side {
		for x := range side {
			relax(x, y, -1, -1)
		}
	}
	for y := side - 1; y >= 0; y-- {
		for x := side - 1; x >= 0; x-- {
			relax(x, y, 1, 1)
		}
	}
	outlineColor, glowColor := colorOr(c.OutlineColor, [4]byte{}), colorOr(c.Color, [4]byte{})
	glow, outline := float64(c.Glow), float64(c.Outline)
	for index, gap := range gaps {
		if gap >= max(glow, outline+1) {
			continue
		}
		to := img.Pix[index*4:]
		var pixel mixer
		if gap > 0 && gap < glow {
			fade := 1 - gap/glow
			pixel.add(glowColor, 0.55*fade*fade)
		}
		if outline > 0 {
			pixel.add(outlineColor, coverage(gap-outline))
		}
		pixel.add([4]byte(to), float64(to[3])/255)
		pixel.write(to)
	}
}
