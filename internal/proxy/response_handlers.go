package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	streamingTailBufferSize = 16384
	streamingChunkSize      = 4 * 1024
)

// handleStreamingResponse handles streaming SSE responses and returns parsed token usage
func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response) *TokenUsage {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		return ps.handleNormalResponse(c, resp)
	}

	// Use a circular buffer to capture the last 16KB of the stream for token parsing
	// Increased from 4KB to handle providers like Volces that send usage in a separate chunk after finish_reason
	tailBuffer := newTailBuffer(streamingTailBufferSize)

	buf := make([]byte, streamingChunkSize)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// Write to tail buffer for later token parsing (do this first)
			tailBuffer.Write(buf[:n])
			// Write to client (ignore errors, client may have disconnected)
			c.Writer.Write(buf[:n])
			flusher.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			logUpstreamError("reading from upstream", err)
			break
		}
	}

	tailData := tailBuffer.Bytes()
	// Parse token usage from the tail of the stream
	return ParseUsageFromStream(tailData)
}

// handleNormalResponse handles non-streaming responses and returns parsed token usage
func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response) *TokenUsage {
	// Read the entire response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logUpstreamError("reading response body", err)
		return nil
	}

	// Check if response is gzip compressed
	var bodyToParse []byte = body
	isGzip := strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") ||
		(len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b)

	if isGzip && len(body) > 2 {
		// Try to decompress for token parsing
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err == nil {
			defer reader.Close()
			decompressed, err := io.ReadAll(reader)
			if err == nil {
				bodyToParse = decompressed
			}
		}
	}

	// Parse token usage from the response
	usage := ParseUsage(bodyToParse)

	// Write the original (possibly compressed) response to the client
	if _, err := c.Writer.Write(body); err != nil {
		logUpstreamError("writing response body", err)
	}

	return usage
}

// tailBuffer is a circular buffer that keeps only the last N bytes
type tailBuffer struct {
	buf     []byte
	maxSize int
	size    int
}

func newTailBuffer(maxSize int) *tailBuffer {
	return &tailBuffer{
		buf:     make([]byte, maxSize),
		maxSize: maxSize,
	}
}

func (tb *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if n > tb.maxSize {
		// If the write is larger than the buffer, only keep the last part
		copy(tb.buf, p[len(p)-tb.maxSize:])
		tb.size = tb.maxSize
		return n, nil
	}

	// Check if we need to wrap around
	remaining := tb.maxSize - tb.size
	if n <= remaining {
		// Fits in the remaining space
		copy(tb.buf[tb.size:], p)
		tb.size += n
	} else {
		shift := n - remaining
		if shift >= tb.size {
			copy(tb.buf, p)
		} else {
			copy(tb.buf, tb.buf[shift:])
			copy(tb.buf[tb.maxSize-n:], p)
		}
		tb.size = tb.maxSize
	}
	return n, nil
}

func (tb *tailBuffer) Bytes() []byte {
	if tb.size < tb.maxSize {
		return tb.buf[:tb.size]
	}
	return tb.buf
}
