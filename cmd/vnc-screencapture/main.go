package main

import (
	"context"
	"flag"
	"fmt"
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
		log.Fatal("Connection failed: %v", err)
	}

	done := make(chan any)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sc
		done <- struct{}{}
	}()

	fmt.Println("Capturing VNC (Press Ctrl+C to end).")
	gifBytes, err := screencapture.RecordGIF(context.Background(), conn, done)
	if err != nil {
		log.Fatal("GIF recording failure: %v", err)
	}

	fmt.Printf("Writing to %v... ", out)
	err = os.WriteFile(out, gifBytes, 0644)
	if err != nil {
		log.Fatal("Failed to write GIF: %v", err)
	}
	fmt.Printf("Done!\n")
}

func Connect(connStr string) (net.Conn, error) {
	return net.Dial("tcp", connStr)
}
