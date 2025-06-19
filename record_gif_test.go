package screencapture

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/und3f/vnc-screencapture/fbupdater"
	vnc "github.com/unistack-org/go-rfb"
)

type Frame = []*vnc.Rectangle

const (
	width      = 10
	height     = 20
	frameDelay = 160 * time.Millisecond
)

var (
	white = vncRGB(255, 255, 255)
	black = vncRGB(0, 0, 0)
	red   = vncRGB(255, 0, 0)
	blue  = vncRGB(0, 0, 255)

	frames = []Frame{
		{
			GenerateFilledRect(0, 0, width, height, black),
		},
		{
			GenerateFilledRect(0, 0, width, height, black),
		},
		{
			GenerateFilledRect(width/2, height/2, width/2, height/2, white),
		},
		{
			GenerateFilledRect(width/2, 0, width/2, height/2, red),
			GenerateFilledRect(0, height/2, width/2, height/2, blue),
		},
	}

	expectedFrameDelay = int(frameDelay.Milliseconds() / 10)
)

func TestStartCapture_success(t *testing.T) {
	serverListener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	if err != nil {
		log.Fatalf("server listen failed: %v", err)
	}

	go func() {
		defer func() {
			if err = serverListener.Close(); err != nil {
				log.Printf("Close failed: %v", err)
			}
		}()

		err = RunVNCMockServer(serverListener)
		if err != nil {
			log.Fatalf("Mock server start failed: %v", err)
		}
	}()

	timeout := 1 * time.Second

	clientConn, err := net.DialTimeout("tcp", serverListener.Addr().String(), timeout)
	if err != nil {
		fmt.Printf("Client Dial failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	doneCh := make(chan any, 1)
	go func() {
		select {
		case <-ctx.Done():
			doneCh <- struct{}{}
		}
	}()

	gif, err := RecordGIFWithOptions(ctx, clientConn, CaptureOptions{
		DoneCh:           doneCh,
		FBUpdaterFactory: fbupdater.OscillatorFBUpdaterFactory(frameDelay),
	})
	if err != nil {
		log.Fatalf("gif record failed: %v", err)
	}

	assert.Equal(t, len(frames)-1, len(gif.Image))
	assert.Equal(t, len(frames)-1, len(gif.Delay))
	for i, frameDelay := range gif.Delay {
		expectedCurrentFrameDelay := expectedFrameDelay
		t.Run(fmt.Sprintf("Delay[%d]", i), func(t *testing.T) {
			assert.InDelta(t, expectedCurrentFrameDelay, frameDelay, 10.0)
		})
	}
}

func RunVNCMockServer(ln *net.TCPListener) error {
	receiveCh := make(chan vnc.ClientMessage, 1)
	sendCh := make(chan vnc.ServerMessage, 1)

	frameInd := 0

	cfg := &vnc.ServerConfig{
		Width:            width,
		Height:           height,
		Handlers:         vnc.DefaultServerHandlers,
		SecurityHandlers: []vnc.SecurityHandler{&vnc.ClientAuthNone{}},
		Encodings:        []vnc.Encoding{&vnc.RawEncoding{}},
		PixelFormat:      vnc.PixelFormat32bit,
		ClientMessageCh:  receiveCh,
		ServerMessageCh:  sendCh,
		Messages:         vnc.DefaultClientMessages,
	}

	c, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}

	conn, err := vnc.NewServerConn(c, cfg)
	if err != nil {
		log.Fatal(err)
	}
	closeConn := func() {
		if err = conn.Close(); err != nil {
			log.Fatal(err)
		}
	}

	defer closeConn()

	go func() {
		for _, h := range cfg.Handlers {
			if err := h.Handle(conn); err != nil {
				if cfg.ErrorCh != nil {
					cfg.ErrorCh <- err
				}
				if err = conn.Close(); err != nil {
					log.Fatal(err)
				}
				return
			}
		}
	}()

	done := false
	for !done {
		select {
		case err := <-cfg.ErrorCh:
			done = true
			log.Fatalf("Server error: %v", err)
		case msg := <-receiveCh:
			switch msg.Type() {
			case vnc.FramebufferUpdateRequestMsgType:
				frame := frames[frameInd]
				cfg.ServerMessageCh <- &vnc.FramebufferUpdate{
					NumRect: uint16(len(frame)),
					Rects:   frame}

				frameInd++
				if frameInd >= len(frames) {
					done = true
				}
			}
		}
	}

	time.Sleep(100 * time.Millisecond)
	return err
}

func vncRGB(r, g, b uint16) *vnc.Color {
	color := vnc.NewColor(vnc.PixelFormat32bit, nil)
	color.R = r
	color.G = g
	color.B = b

	return color
}

func GenerateFilledRect(x, y, width, height uint16, color *vnc.Color) *vnc.Rectangle {
	colors := make([]vnc.Color, width*height)
	for i := range colors {
		colors[i] = *color
	}

	return &vnc.Rectangle{
		X:       x,
		Y:       y,
		Width:   width,
		Height:  height,
		EncType: vnc.EncRaw,
		Enc: &vnc.RawEncoding{
			Colors: colors,
		},
	}
}
