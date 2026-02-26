package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gpt-load/internal/channel"
)

const (
	streamingTailBufferSize = 16384
	streamingChunkSize      = 4 * 1024
)

// handleStreamingResponse handles streaming SSE responses and returns token usage and generation times
func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response, channelHandler channel.ChannelProxy, model string, requestBody []byte) (*TokenUsage, int64, int64) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	contentEncoding := resp.Header.Get("Content-Encoding")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		usage, _, _ := ps.handleNormalResponse(c, resp, channelHandler, model, requestBody)
		return usage, 0, 0
	}

	tailBuffer := newTailBuffer(streamingTailBufferSize)

	// 记录首末 token 时间
	var firstTokenTime int64
	var lastTokenTime int64
	hasFirstToken := false

	// Check if we need to manually decompress the response body
	// Go's http.Client auto-decompresses gzip/deflate but NOT brotli (br)
	var bodyReader io.Reader = resp.Body
	if contentEncoding == "br" {
		// brotli.Reader doesn't implement io.ReadCloser, use it directly as io.Reader
		bodyReader = brotli.NewReader(resp.Body)
		// Remove Content-Encoding header since we're decompressing
		c.Header("Content-Encoding", "")
	} else if contentEncoding == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			logrus.Warnf("Failed to create gzip reader: %v", err)
		} else {
			defer gzReader.Close()
			bodyReader = gzReader
			// Remove Content-Encoding header since we're decompressing
			c.Header("Content-Encoding", "")
		}
	}

	buf := make([]byte, streamingChunkSize)
	for {
		n, err := bodyReader.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			// 检查是否包含 content，记录时间
			if containsContent(chunk) {
				now := time.Now().UnixMilli()
				if !hasFirstToken {
					firstTokenTime = now
					hasFirstToken = true
				}
				lastTokenTime = now
			}

			tailBuffer.Write(chunk)
			c.Writer.Write(chunk)
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

	// Try to parse usage from upstream response first
	channelType := channelHandler.GetChannelType()
	usage := ParseUsageFromStream(tailBuffer.Bytes(), channelType)

	// If parsing failed, estimate tokens as fallback
	if usage == nil {
		usage = estimateTokensFromStream(model, requestBody, tailBuffer.Bytes())
	}

	return usage, firstTokenTime, lastTokenTime
}

// handleNormalResponse handles non-streaming responses and returns token usage
func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response, channelHandler channel.ChannelProxy, model string, requestBody []byte) (*TokenUsage, int64, int64) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logUpstreamError("reading response body", err)
		return nil, 0, 0
	}

	// For parsing usage, we may need to decompress brotli
	// Go's http.Client auto-decompresses gzip/deflate but NOT brotli
	var bodyToParse []byte = body
	contentEncoding := resp.Header.Get("Content-Encoding")

	if contentEncoding == "br" && len(body) > 0 {
		// Brotli compressed - decompress for parsing
		reader := brotli.NewReader(bytes.NewReader(body))
		decompressed, err := io.ReadAll(reader)
		if err == nil {
			bodyToParse = decompressed
		}
	}

	channelType := channelHandler.GetChannelType()
	usage := ParseUsage(bodyToParse, channelType)

	if usage == nil {
		usage = estimateTokens(model, requestBody, bodyToParse)
	}

	if _, err := c.Writer.Write(body); err != nil {
		logUpstreamError("writing response body", err)
	}

	return usage, 0, 0
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
