package screencapture

import (
	"bytes"
	"context"
	"image/gif"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	vnc "github.com/unistack-org/go-rfb"
)

type Frame = []*vnc.Rectangle

const (
	width  = 10
	height = 20
	delay  = time.Second / 10
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
			GenerateFilledRect(width/2, height/2, width/2, height/2, white),
		},
		{
			GenerateFilledRect(width/2, 0, width/2, height/2, red),
			GenerateFilledRect(0, height/2, width/2, height/2, blue),
		},
	}
)

func TestStartCapture_success(t *testing.T) {
	checkErr := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	serverListener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 0})
	checkErr(err)

	doneCh := make(chan any, 1)
	go func() {
		defer serverListener.Close()

		err = RunVNCMockServer(serverListener)
		if err != nil {
			log.Fatal(err)
		}
	}()

	timeout := 1 * time.Second

	clientConn, err := net.DialTimeout("tcp", serverListener.Addr().String(), timeout)
	checkErr(err)
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	res, err := RecordGIF(ctx, clientConn, doneCh)
	checkErr(err)

	buf := bytes.NewBuffer(res)
	gif, err := gif.DecodeAll(buf)
	checkErr(err)

	assert.Equal(t, 3, len(gif.Image))
	for _, delay := range gif.Delay {
		assert.InDelta(t, 10, delay, 3.0)
	}
}

func RunVNCMockServer(ln *net.TCPListener) error {
	chServer := make(chan vnc.ClientMessage)
	chClient := make(chan vnc.ServerMessage)

	frameInd := 0

	cfg := &vnc.ServerConfig{
		Width:            width,
		Height:           height,
		Handlers:         vnc.DefaultServerHandlers,
		SecurityHandlers: []vnc.SecurityHandler{&vnc.ClientAuthNone{}},
		Encodings:        []vnc.Encoding{&vnc.RawEncoding{}},
		PixelFormat:      vnc.PixelFormat32bit,
		ClientMessageCh:  chServer,
		ServerMessageCh:  chClient,
		Messages:         vnc.DefaultClientMessages,
	}

	c, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}

	conn, err := vnc.NewServerConn(c, cfg)
	if err != nil {
		cfg.ErrorCh <- err
		log.Fatal(err)
	}

	go func() {
		for _, h := range cfg.Handlers {
			if err := h.Handle(conn); err != nil {
				if cfg.ErrorCh != nil {
					cfg.ErrorCh <- err
				}
				conn.Close()
				return
			}
		}
	}()

	done := false
	for !done {
		select {
		case msg := <-chServer:
			switch msg.Type() {
			case vnc.FramebufferUpdateRequestMsgType:
				frame := frames[frameInd]
				time.Sleep(delay)
				cfg.ServerMessageCh <- &vnc.FramebufferUpdate{
					NumRect: uint16(len(frame)),
					Rects:   frame}

				frameInd++
				if frameInd >= len(frames) {
					done = true
					break
				}
			}
		}
	}

	// Ensure messages are sent
	time.Sleep(delay)
	conn.Close()

	return nil
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
