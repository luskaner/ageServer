package tray

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"sync"
)

// embeddedAppIcon is the same picture the Wails GUI shows in its window
// (gui/server/config/build/appicon.png), embedded here so the agent's tray
// icon matches the application. A user can tell at a glance which program owns
// the notification area.
//
// The source PNG is 1024 px; tray icons end up at sixteen physical pixels, so we
// downscale it to iconSize and let the platform scale further if needed.
//
// A tray icon is a binary file that has to be sourced, licensed, regenerated
// whenever the palette moves and, on the platforms that keep icons in resources,
// compiled into the executable. Embedding it keeps the binary self-contained and
// the icon in step with the GUI by construction.

//go:embed embedded_appicon.png
var embeddedAppIcon []byte

// embed is referenced by the //go:embed directive above.
var _ embed.FS

// iconSize is the size the icon is rendered at. The platform scales it to
// whatever the notification area asks for, and a source larger than that keeps
// the downscale honest on a high density display.
const iconSize = 32

// iconPNG is the rendered icon, encoded once.
var iconPNG = sync.OnceValue(func() []byte {
	src, err := png.Decode(bytes.NewReader(embeddedAppIcon))
	if err != nil {
		// The embedded PNG was validated at build time; if decoding fails here
		// something is seriously wrong. Return nothing rather than a broken
		// icon: the tray comes up with no picture rather than not at all.
		return nil
	}
	small := downscale(src, iconSize, iconSize)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, small); err != nil {
		return nil
	}
	return buffer.Bytes()
})

// downscale scales src to the given width and height by averaging each source
// block that maps to one destination pixel. 1024 divides evenly by 32, so each
// destination pixel is the average of exactly 32x32 source pixels.
func downscale(src image.Image, width, height int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			x0 := dx * srcW / width
			x1 := (dx + 1) * srcW / width
			y0 := dy * srcH / height
			y1 := (dy + 1) * srcH / height
			var r, g, b, a uint32
			n := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, ca := src.At(x, y).RGBA()
					r += cr >> 8
					g += cg >> 8
					b += cb >> 8
					a += ca >> 8
					n++
				}
			}
			if n > 0 {
				offset := (dy*width + dx) * 4
				dst.Pix[offset] = uint8(r / uint32(n))
				dst.Pix[offset+1] = uint8(g / uint32(n))
				dst.Pix[offset+2] = uint8(b / uint32(n))
				dst.Pix[offset+3] = uint8(a / uint32(n))
			}
		}
	}
	return dst
}
