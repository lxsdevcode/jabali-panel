// Package api — LibreSpeed-style connection speed test backing the Server
// Status "Speed Test" card (GH #252).
//
// Three tiny admin-only endpoints measure the link between the operator's
// browser and the panel host — the same three primitives LibreSpeed uses
// (garbage download, sink upload, empty ping):
//
//	GET  /admin/speedtest/ping                 -> 204, no body (latency/jitter)
//	GET  /admin/speedtest/download?bytes=N      -> N bytes of incompressible data
//	POST /admin/speedtest/upload                -> drains+discards the body
//
// No agent, DB, or Redis dependency — pure HTTP. RequireAdmin gates all
// three; the download/upload sizes are capped so the endpoints can't be
// turned into an amplification or memory-exhaustion lever.
package api

import (
	"crypto/rand"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/middleware"
)

const (
	// Per-request transfer ceiling for both directions. The client streams
	// several parallel requests over a fixed time window, so a generous
	// per-request cap is fine; this only bounds a single abusive call.
	speedtestMaxBytes     = 100 << 20 // 100 MiB
	speedtestDefaultBytes = 25 << 20  // 25 MiB
	speedtestChunk        = 64 << 10  // 64 KiB write unit
)

// speedtestGarbage is a one-time random buffer reused for every download
// write. Random so a compressing hop (gzip) can't shrink it and inflate the
// measured throughput; filled once because regenerating per request wastes
// CPU and the bytes on the wire are what matter.
var speedtestGarbage = func() []byte {
	b := make([]byte, speedtestChunk)
	// crypto/rand keeps staticcheck happy (math/rand.Read is deprecated); the
	// bytes are throwaway filler, not a secret, but Read here effectively
	// never fails for a 64 KiB buffer.
	if _, err := rand.Read(b); err != nil {
		panic("speedtest: seed garbage buffer: " + err.Error())
	}
	return b
}()

// RegisterAdminSpeedtestRoutes mounts the three speed-test endpoints under
// the RequireAdmin group (GH #252).
func RegisterAdminSpeedtestRoutes(g *gin.RouterGroup) {
	grp := g.Group("/admin/speedtest")
	grp.Use(middleware.RequireAdmin())
	grp.GET("/ping", speedtestPing)
	grp.GET("/download", speedtestDownload)
	grp.POST("/upload", speedtestUpload)
}

// speedtestPing is the latency/jitter probe — empty 204, no caching.
func speedtestPing(c *gin.Context) {
	noStore(c)
	c.Status(http.StatusNoContent)
}

// speedtestDownload streams up to bytes of incompressible data without
// allocating the whole payload — it writes the shared garbage buffer in a
// loop and flushes so the client sees a steady stream.
func speedtestDownload(c *gin.Context) {
	noStore(c)
	remaining := speedtestDefaultBytes
	if q := c.Query("bytes"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 0 {
			remaining = n
		}
	}
	if remaining > speedtestMaxBytes {
		remaining = speedtestMaxBytes
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(remaining))
	flusher, _ := c.Writer.(http.Flusher)
	for remaining > 0 {
		n := speedtestChunk
		if remaining < n {
			n = remaining
		}
		if _, err := c.Writer.Write(speedtestGarbage[:n]); err != nil {
			return // client aborted (expected — it stops at the time window)
		}
		remaining -= n
		if flusher != nil {
			flusher.Flush()
		}
	}
}

// speedtestUpload drains the request body into the sink, bounded so a
// runaway upload can't exhaust memory. The byte count isn't echoed — the
// client times its own send.
func speedtestUpload(c *gin.Context) {
	noStore(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, speedtestMaxBytes)
	_, _ = io.Copy(io.Discard, c.Request.Body)
	c.Status(http.StatusOK)
}

func noStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
}
