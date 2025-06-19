package screencapture

import (
	"image"
	"image/color"
	"maps"
	"slices"

	vnc "github.com/unistack-org/go-rfb"
)

// VNC utilities

func vncRawColorToPalettedImage(colors []vnc.Color, imageSize image.Rectangle) *image.Paletted {
	palette := getVNCRawColorPalette(colors)
	frame := image.NewPaletted(imageSize, palette)
	for y := imageSize.Min.Y; y <= imageSize.Max.Y; y++ {
		for x := imageSize.Min.X; x <= imageSize.Max.X; x++ {
			i := y*imageSize.Max.X + x
			frame.Set(x, y, vncRawColorToImageColor(colors[i]))
		}
	}

	return frame
}

func getVNCRawColorPalette(colors []vnc.Color) color.Palette {
	paletteMap := make(map[color.Color]any)
	for _, vncColor := range colors {
		color := vncRawColorToImageColor(vncColor)
		paletteMap[color] = struct{}{}
	}

	return slices.Collect(maps.Keys(paletteMap))
}

func vncRawColorToImageColor(vncColor vnc.Color) color.RGBA {
	return color.RGBA{uint8(vncColor.R), uint8(vncColor.G), uint8(vncColor.B), 255}
}

func vNCFrameUpdate(frame []vnc.Color, imageWidth int, rects []*vnc.Rectangle) {
	for _, rect := range rects {
		width := int(rect.Width)
		for y := 0; y < int(rect.Height); y++ {
			subframe := rect.Enc.(*vnc.RawEncoding).Colors
			origI := (int(rect.Y)+y)*imageWidth + int(rect.X)
			i := int(y) * int(rect.Width)
			copy(frame[origI:origI+width], subframe[i:i+width])
		}
	}
}
