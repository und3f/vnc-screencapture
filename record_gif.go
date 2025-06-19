package screencapture

import (
	"context"
	"fmt"
	"image/gif"
	"net"
)

// RecordGIF initializes VNC connection, request screen updates until either
// context canceled, doneCh received, or VNC connection is closed using defaultOptions.
func RecordGIF(ctx context.Context, conn net.Conn, doneCh chan any) (*gif.GIF, error) {
	var options = defaultOptions
	options.DoneCh = doneCh

	return RecordGIFWithOptions(ctx, conn, options)
}

func RecordGIFWithOptions(ctx context.Context, conn net.Conn, options CaptureOptions) (*gif.GIF, error) {
	recorder, err := ConnectWithOptions(ctx, conn, options)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := recorder.Close(); err != nil {
			fmt.Printf("Failed to close Screen Capture: %v", err)
		}
	}()

	recorder.Record(options.DoneCh)

	return recorder.RenderGIF()
}
