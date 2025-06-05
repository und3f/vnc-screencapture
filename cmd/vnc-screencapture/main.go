/*
vnc-screencapture captures VNC screen and writes captured frames to GIF file.

	vnc-screencapture -vnc localhost:5900 -out screen-capture.gif

Usage

	vnc-screencapture [flag]
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"image/gif"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	screencapture "github.com/und3f/vnc-screencapture"
)

func main() {
	var connStr, out string

	flag.StringVar(&connStr, "vnc", "localhost:5700", "VNC connection details")
	flag.StringVar(&out, "out", "vnc-record.gif", "Output file")

	flag.Parse()

	conn, err := Connect(connStr)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}

	done := make(chan any)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sc
		done <- struct{}{}
	}()

	fmt.Println("Capturing VNC (Press Ctrl+C to end).")
	gifData, err := screencapture.RecordGIF(context.Background(), conn, done)
	if err != nil {
		log.Fatalf("GIF recording failure: %v", err)
	}
	fmt.Printf("Writing to %v... ", out)

	f, err := os.Create(out)
	if err != nil {
		log.Fatalf("Failed create GIF file: %v", err)
	}

	err = gif.EncodeAll(f, gifData)
	if err != nil {
		log.Fatalf("Failed to write GIF: %v", err)
	}

	if err := f.Close(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Done!\n")
}

func Connect(connStr string) (net.Conn, error) {
	return net.Dial("tcp", connStr)
}
