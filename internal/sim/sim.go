// Package sim shows the dashboard in a web browser on the PC: clicks act as
// touches. It listens on the local machine only.
package sim

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/screen"
)

type Display struct {
	mu    sync.Mutex
	panel *image.Gray
	taps  chan image.Point
	srv   *http.Server
}

const page = `<!doctype html><meta charset="utf-8"><title>Kobo simulator</title>
<body style="margin:0;background:#777">
<img id="s" src="/screen.png" style="display:block;max-width:100vw;max-height:100vh;cursor:pointer">
<script>
const s = document.getElementById("s");
setInterval(() => { const i = new Image(); i.onload = () => s.src = i.src; i.src = "/screen.png?" + Date.now(); }, 300);
s.onclick = e => {
  const k = s.naturalWidth / s.clientWidth;
  fetch("/tap?x=" + Math.floor(e.offsetX * k) + "&y=" + Math.floor(e.offsetY * k), {method: "POST"});
};
</script>`

// Start serves the simulator on addr (host:port, loopback only).
func Start(addr string, orientation string) (*Display, <-chan image.Point, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, nil, err
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		return nil, nil, fmt.Errorf("the simulator only listens on a loopback address, such as 127.0.0.1:8080")
	}
	w, h := 758, 1024
	if orientation == "landscape" || orientation == "landscape_inverted" {
		w, h = h, w
	}
	d := &Display{panel: image.NewGray(image.Rect(0, 0, w, h)), taps: make(chan image.Point, 8)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Write([]byte(page))
	})
	mux.HandleFunc("GET /screen.png", func(rw http.ResponseWriter, _ *http.Request) {
		var buf bytes.Buffer
		d.mu.Lock()
		err := png.Encode(&buf, d.panel)
		d.mu.Unlock()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "image/png")
		rw.Header().Set("Cache-Control", "no-store")
		rw.Write(buf.Bytes())
	})
	mux.HandleFunc("POST /tap", func(rw http.ResponseWriter, r *http.Request) {
		// Other web pages must not be able to press buttons.
		if s := r.Header.Get("Sec-Fetch-Site"); s != "" && s != "same-origin" {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		x, errX := strconv.Atoi(r.URL.Query().Get("x"))
		y, errY := strconv.Atoi(r.URL.Query().Get("y"))
		if errX != nil || errY != nil {
			http.Error(rw, "bad coordinates", http.StatusBadRequest)
			return
		}
		select {
		case d.taps <- image.Pt(x, y):
		default:
		}
	})

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	d.srv = &http.Server{Handler: mux}
	go d.srv.Serve(l)
	return d, d.taps, nil
}

func (d *Display) Size() (int, int) { return d.panel.Rect.Dx(), d.panel.Rect.Dy() }

func (d *Display) Show(img *image.Gray, r image.Rectangle, _ screen.Mode) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	draw.Draw(d.panel, r, img, r.Min, draw.Src)
	return nil
}

func (d *Display) Close() error { return d.srv.Close() }
