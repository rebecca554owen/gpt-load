package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gpt-load/internal/channel"
)

const (
	streamingTailBufferSize = 16384
	streamingChunkSize      = 4 * 1024
)

// handleStreamingResponse 处理流式 SSE 响应并返回 token 使用情况
func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response, channelHandler channel.ChannelProxy, model string, requestBody []byte) *TokenUsage {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	contentEncoding := resp.Header.Get("Content-Encoding")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		return ps.handleNormalResponse(c, resp, channelHandler, model, requestBody)
	}

	tailBuffer := newTailBuffer(streamingTailBufferSize)

	// 检查是否需要手动解压响应体
	// Go 的 http.Client 自动解压 gzip/deflate，但不会解压 brotli (br)
	var bodyReader io.Reader = resp.Body
	if contentEncoding == "br" {
		// brotli.Reader 未实现 io.ReadCloser，直接作为 io.Reader 使用
		bodyReader = brotli.NewReader(resp.Body)
		// 移除 Content-Encoding 头，因为我们正在解压
		c.Header("Content-Encoding", "")
	} else if contentEncoding == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			logrus.Warnf("Failed to create gzip reader: %v", err)
		} else {
			defer gzReader.Close()
			bodyReader = gzReader
			// 移除 Content-Encoding 头，因为我们正在解压
			c.Header("Content-Encoding", "")
		}
	}

	buf := make([]byte, streamingChunkSize)
	for {
		n, err := bodyReader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
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

	// 首先尝试从上游响应解析使用情况
	channelType := channelHandler.GetChannelType()
	usage := ParseUsageFromStream(tailBuffer.Bytes(), channelType)

	// 如果解析失败，估算 token 作为回退
	if usage == nil {
		usage = estimateTokensFromStream(model, requestBody, tailBuffer.Bytes())
	}

	return usage
}

// handleNormalResponse 处理非流式响应并返回 token 使用情况
func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response, channelHandler channel.ChannelProxy, model string, requestBody []byte) *TokenUsage {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logUpstreamError("reading response body", err)
		return nil
	}

	// 为了解析使用情况，可能需要解压 brotli
	// Go 的 http.Client 自动解压 gzip/deflate，但不会解压 brotli
	var bodyToParse []byte = body
	contentEncoding := resp.Header.Get("Content-Encoding")

	if contentEncoding == "br" && len(body) > 0 {
		// Brotli 压缩 - 解压以进行解析
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

	return usage
}

// tailBuffer 是一个环形缓冲区，仅保留最后 N 个字节
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
		// 如果写入大于缓冲区，仅保留最后部分
		copy(tb.buf, p[len(p)-tb.maxSize:])
		tb.size = tb.maxSize
		return n, nil
	}

	// 检查是否需要回绕
	remaining := tb.maxSize - tb.size
	if n <= remaining {
		// 适合剩余空间
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
