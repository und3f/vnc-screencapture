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
	"log"
	"net"
	"time"

	"github.com/und3f/vnc-screencapture/fbupdater"
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

	images                 []*image.Paletted
	frameRequestTimestamps []time.Time
	delays                 []int

	fbUpdater fbupdater.FBUpdater
}

type CaptureOptions struct {
	FBUpdaterFactory fbupdater.FBUpdaterFactory
	DoneCh           chan any
}

// Connect initializes VNC connection and starts message processing goroutine. Use default parameters.
func Connect(ctx context.Context, conn net.Conn) (ScreenCapture, error) {
	return ConnectWithOptions(ctx, conn, defaultOptions)
}

const (
	maximumQueuedFrames = 4
)

var (
	defaultOptions = CaptureOptions{
		FBUpdaterFactory: fbupdater.NewOnFrameReceivedFBUpdater,
	}
)

// Connect initializes VNC connection and starts message processing goroutine.
func ConnectWithOptions(ctx context.Context, conn net.Conn, options CaptureOptions) (ScreenCapture, error) {
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

	// Receive initial update buffer response
	<-cchServer

	sc := &screenCaptureImpl{
		conn: vncConn,
		cfg:  ccfg,
	}

	sc.fbUpdater = options.FBUpdaterFactory(sc)

	return sc, nil
}

func (sc *screenCaptureImpl) Record(doneCh chan any) error {
	if sc.conn == nil {
		return errors.New("not connected")
	}

	conn := sc.conn

	imageSize := image.Rectangle{image.Point{0, 0}, image.Point{int(conn.Width()), int(conn.Height())}}
	var rfbFrame []vnc.Color

	sc.fbUpdater.Start()
	defer sc.fbUpdater.Stop()
	defer sc.recordFBUpdateRequestTimestamp()

	done := false
	for !done {
		select {
		case <-sc.cfg.ErrorCh:
			done = true
			continue
		case msg := <-sc.cfg.ServerMessageCh:
			sc.fbUpdater.OnFrameReceived()
			rfbUpdateMsg := msg.(*vnc.FramebufferUpdate)
			if rfbFrame == nil {
				if !sc.isFullScreenImage(rfbUpdateMsg) {
					// Skip incremental screen updates until we receive full
					continue
				}
				rfbFrame = make([]vnc.Color, int(conn.Width())*int(conn.Height()))
			}
			vNCFrameUpdate(rfbFrame, int(conn.Width()), rfbUpdateMsg.Rects)
			sc.images = append(sc.images, vncRawColorToPalettedImage(rfbFrame, imageSize))
		case <-doneCh:
			done = true
		}
	}

	return nil
}

func (sc *screenCaptureImpl) RenderGIF() (*gif.GIF, error) {

	sc.delays = make([]int, len(sc.images))

	for i := range sc.delays {
		sc.delays[i] = int(sc.frameRequestTimestamps[i+1].Sub(sc.frameRequestTimestamps[i]).Milliseconds() / 10)
	}
	sc.frameRequestTimestamps = nil

	gifRes := &gif.GIF{
		Image: sc.images,
		Delay: sc.delays,
	}

	sc.images = nil
	sc.delays = nil
	sc.frameRequestTimestamps = nil

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

func (sc *screenCaptureImpl) UpdateFB(incremental bool) {
	var inc uint8
	if incremental {
		inc = 1
	}

	// Create Update Request
	fbur := &vnc.FramebufferUpdateRequest{
		Inc:    inc,
		X:      0,
		Y:      0,
		Width:  sc.conn.Width(),
		Height: sc.conn.Height(),
	}

	// Initial FB request (no increment)
	sc.cfg.ClientMessageCh <- fbur

	sc.recordFBUpdateRequestTimestamp()
}

func (sc *screenCaptureImpl) recordFBUpdateRequestTimestamp() {
	sc.frameRequestTimestamps = append(sc.frameRequestTimestamps, time.Now())
}
