package screencapture

import (
	"context"
	"fmt"
	"image/gif"
	"net"
)

// RecordGIF initializes VNC connection, request screen updates until either
// context canceled, doneCh received, or VNC connection is closed.
func RecordGIF(ctx context.Context, conn net.Conn, doneCh chan any) (*gif.GIF, error) {
	recorder, err := Connect(ctx, conn)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := recorder.Close(); err != nil {
			fmt.Printf("Failed to close Screen Capture: %v", err)
		}
	}()

	if err := recorder.Record(doneCh); err != nil {
		return nil, err
	}

	return recorder.RenderGIF()
}
