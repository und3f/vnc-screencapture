[![Go Reference](https://pkg.go.dev/badge/github.com/und3f/vnc-screencapture.svg)](https://pkg.go.dev/github.com/und3f/vnc-screencapture)

# vnc-screencapture
Go VNC screen capture API and CLI.

## Command line utility

Install with `go install`:
```
go install github.com/und3f/vnc-screencapture/cmd/vnc-screencapture
```

```
vnc-screencapture -vnc localhost:5900
```

## API
Example of API usage for capturing few seconds from VNC connection:

```go
conn, _ := net.Dial("tcp", "localhost:5900")

recorder, _ := screencapture.Connect(context.Background(), conn)

defer recorder.Close()

doneCh := make(chan any)
go func() {
    time.Sleep(2 * time.Second)
    doneCh <- struct{}{}
}()

recorder.Record(doneCh)

f, _ := os.Create("out.gif")
defer f.Close()

gifData, _ := recorder.RenderGIF()
gif.EncodeAll(f, gifData)
```
