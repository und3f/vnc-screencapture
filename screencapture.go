/*
Package vnc-screencapture provides API for capturing remote screen over VNC connection.
*/
package screencapture // import "github.com/und3f/vnc-screencapture"

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"io"
	"log"
	"net"
	"time"

	vnc "github.com/unistack-org/go-rfb"
)

// RecordGIF initializes VNC connection, request screen updates until either
// context canceled, doneCh received, or VNC connection is closed.
func RecordGIF(ctx context.Context, conn net.Conn, doneCh chan any) (*gif.GIF, error) {
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

	cc, err := vnc.Connect(ctx, conn, ccfg)
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
				vNCFrameUpdate(rfbFrame, int(cc.Width()), rfbUpdateMsg.Rects)
				delays = append(delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)
			}
			images = append(images, vncRawColorToPalettedImage(rfbFrame, imageSize))
			lastFrameAt = time.Now()
			cchClient <- fbur
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-doneCh:
			done = true
		}
	}

	delays = append(delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)
	return &gif.GIF{
		Image: images,
		Delay: delays,
	}, nil
}
