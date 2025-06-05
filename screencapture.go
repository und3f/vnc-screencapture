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

// ScreenCapture is the interface for capturing VNC screen frames.
//
// Record starts recording screen frames and could be stopped with done
// channel). Same connection may be reused for recording multiple times.
//
// RenderGIF returns GIF image data and clears captured frames.
type ScreenCapture interface {
	Record(done chan any) error
	RenderGIF() (*gif.GIF, error)
	Close() error
}

type screenCaptureImpl struct {
	conn *vnc.ClientConn
	cfg  *vnc.ClientConfig

	images []*image.Paletted
	delays []int
}

// Connect initializes VNC connection and starts message processing goroutine.
func Connect(ctx context.Context, conn net.Conn) (ScreenCapture, error) {
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

	vncConn, err := vnc.Connect(ctx, conn, ccfg)
	if err != nil {
		return nil, fmt.Errorf("VNC connection negotiation failed: %v", err)
	}

	go func() {
		err := (&vnc.DefaultClientMessageHandler{}).Handle(vncConn)
		if err != nil {
			log.Fatal(err)
		}
	}()

	return &screenCaptureImpl{
		conn: vncConn,
		cfg:  ccfg,
	}, nil
}

func (sc *screenCaptureImpl) Record(doneCh chan any) error {
	if sc.conn == nil {
		return errors.New("not connected")
	}

	conn := sc.conn

	imageSize := image.Rectangle{image.Point{0, 0}, image.Point{int(conn.Width()), int(conn.Height())}}
	lastFrameAt := time.Now()
	var rfbFrame []vnc.Color

	// Create Update Request
	fbur := &vnc.FramebufferUpdateRequest{
		Inc:    0,
		X:      0,
		Y:      0,
		Width:  conn.Width(),
		Height: conn.Height(),
	}

	// Initial FB request (no increment)
	sc.cfg.ClientMessageCh <- fbur

	done := false
	for !done {
		select {
		case err := <-sc.cfg.ErrorCh:
			if errors.Is(err, io.EOF) {
				done = true
				break
			}
			return err
		case msg := <-sc.cfg.ServerMessageCh:
			rfbUpdateMsg := msg.(*vnc.FramebufferUpdate)
			if rfbFrame == nil {
				if !sc.isFullScreenImage(rfbUpdateMsg) {
					// Skip partial screen update until we receive full
					continue
				}

				rfbFrame = rfbUpdateMsg.Rects[0].Enc.(*vnc.RawEncoding).Colors
				fbur.Inc = 1
			} else {
				vNCFrameUpdate(rfbFrame, int(conn.Width()), rfbUpdateMsg.Rects)
				sc.delays = append(sc.delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)
			}
			sc.images = append(sc.images, vncRawColorToPalettedImage(rfbFrame, imageSize))
			lastFrameAt = time.Now()
			sc.cfg.ClientMessageCh <- fbur
		case <-doneCh:
			done = true
		}
	}

	sc.delays = append(sc.delays, int(time.Since(lastFrameAt)/time.Millisecond)/10)

	return nil
}

func (sc *screenCaptureImpl) RenderGIF() (*gif.GIF, error) {
	gifRes := &gif.GIF{
		Image: sc.images,
		Delay: sc.delays,
	}

	sc.images = nil
	sc.delays = nil

	return gifRes, nil
}

func (sc *screenCaptureImpl) Close() error {
	return sc.conn.Close()
}

func (sc *screenCaptureImpl) isFullScreenImage(msg *vnc.FramebufferUpdate) bool {
	if len(msg.Rects) != 1 {
		return false
	}

	rect := msg.Rects[0]

	return rect.X == 0 && rect.Y == 0 && rect.Width == sc.conn.Width() && rect.Height == sc.conn.Height()
}
