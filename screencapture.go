package screencapture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"log"
	"maps"
	"net"
	"slices"
	"time"

	vnc "github.com/unistack-org/go-rfb"
)

func RecordGIF(ctx context.Context, con net.Conn, doneCh chan any) ([]byte, error) {
	cchServer := make(chan vnc.ServerMessage, 1)
	cchClient := make(chan vnc.ClientMessage, 1)
	errorCh := make(chan error, 1)

	ccfg := &vnc.ClientConfig{
		SecurityHandlers: []vnc.SecurityHandler{&vnc.ClientAuthNone{}},
		PixelFormat:      vnc.PixelFormat32bit,
		ClientMessageCh:  cchClient,
		ServerMessageCh:  cchServer,
		Messages:         vnc.DefaultServerMessages,
		Encodings:        []vnc.Encoding{&vnc.RawEncoding{}},
		ErrorCh:          errorCh,
		Handlers: []vnc.Handler{
			&vnc.DefaultClientVersionHandler{},
			&vnc.DefaultClientSecurityHandler{},
			&vnc.DefaultClientClientInitHandler{},
			&vnc.DefaultClientServerInitHandler{},
		},
	}

	cc, err := vnc.Connect(ctx, con, ccfg)
	if err != nil {
		return nil, fmt.Errorf("VNC connection negotiation failed: %v", err)
	}

	go func() {
		err := (&vnc.DefaultClientMessageHandler{}).Handle(cc)
		if err != nil {
			log.Fatal(err)
		}
	}()

	imageSize := image.Rectangle{image.Point{0, 0}, image.Point{int(cc.Width()), int(cc.Height())}}
	lastFrameAt := time.Now()
	var rfbFrame []vnc.Color
	var delays []int
	var images []*image.Paletted

	// Create Update Request
	fbur := &vnc.FramebufferUpdateRequest{
		Inc:    1,
		X:      0,
		Y:      0,
		Width:  cc.Width(),
		Height: cc.Height(),
	}

	done := false
	for !done {
		select {
		case err := <-errorCh:
			if errors.Is(err, io.EOF) {
				done = true
				break
			}
			fmt.Println(err)
		case msg := <-cchServer:
			rfbUpdateMsg := msg.(*vnc.FramebufferUpdate)
			if rfbFrame == nil {
				rfbFrame = rfbUpdateMsg.Rects[0].Enc.(*vnc.RawEncoding).Colors
			} else {
				VNCFrameUpdate(rfbFrame, int(cc.Width()), rfbUpdateMsg.Rects)
				delays = append(delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)
			}
			images = append(images, VncRawColorToPalettedImage(rfbFrame, imageSize))
			lastFrameAt = time.Now()
			cchClient <- fbur
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-doneCh:
			done = true
		}
	}

	delays = append(delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)

	buf := new(bytes.Buffer)

	err = gif.EncodeAll(buf, &gif.GIF{
		Image: images,
		Delay: delays,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func VncRawColorToPalettedImage(colors []vnc.Color, imageSize image.Rectangle) *image.Paletted {
	palette := GetVNCRawColorPalette(colors)
	frame := image.NewPaletted(imageSize, palette)
	for y := imageSize.Min.Y; y < imageSize.Max.Y; y++ {
		for x := imageSize.Min.X; x < imageSize.Max.X; x++ {
			i := y*imageSize.Max.X + x
			frame.Set(x, y, VNCRawColorToImageColor(colors[i]))
		}
	}

	return frame
}

func GetVNCRawColorPalette(colors []vnc.Color) color.Palette {
	paletteMap := make(map[color.Color]any)
	for _, vncColor := range colors {
		color := VNCRawColorToImageColor(vncColor)
		paletteMap[color] = struct{}{}
	}

	return slices.Collect(maps.Keys(paletteMap))
}

func VNCRawColorToImageColor(vncColor vnc.Color) color.RGBA {
	return color.RGBA{uint8(vncColor.R), uint8(vncColor.G), uint8(vncColor.B), 255}
}

func VNCFrameUpdate(frame []vnc.Color, imageWidth int, rects []*vnc.Rectangle) {
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
