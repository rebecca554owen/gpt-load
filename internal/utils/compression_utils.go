package utils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/sirupsen/logrus"
)

// Decompressor 定义不同解压算法的接口
type Decompressor interface {
	Decompress(data []byte) ([]byte, error)
}

// decompressorRegistry 存储所有已注册的重压器
var decompressorRegistry = make(map[string]Decompressor)

// init 注册默认重压器
func init() {
	RegisterDecompressor("gzip", new(GzipDecompressor))
	RegisterDecompressor("br", new(BrotliDecompressor))
	RegisterDecompressor("deflate", new(DeflateDecompressor))
	RegisterDecompressor("zstd", new(ZstdDecompressor))
}

// RegisterDecompressor 允许注册新的解压算法
func RegisterDecompressor(encoding string, decompressor Decompressor) {
	decompressorRegistry[encoding] = decompressor
	logrus.Debugf("Registered decompressor for encoding: %s", encoding)
}

// DecompressResponse 根据 Content-Encoding 标头自动解压响应数据
func DecompressResponse(contentEncoding string, data []byte) ([]byte, error) {
	// 如果未指定编码或数据为空，直接返回
	if contentEncoding == "" || len(data) == 0 {
		return data, nil
	}

	// 查找重压器
	decompressor, exists := decompressorRegistry[contentEncoding]
	if !exists {
		logrus.Warnf("No decompressor registered for encoding '%s', returning original data", contentEncoding)
		return data, nil
	}

	// 解压
	decompressed, err := decompressor.Decompress(data)
	if err != nil {
		logrus.WithError(err).Warnf("Failed to decompress with '%s', returning original data", contentEncoding)
		return data, nil
	}

	logrus.Debugf("Successfully decompressed %d bytes -> %d bytes using '%s'",
		len(data), len(decompressed), contentEncoding)
	return decompressed, nil
}

// GzipDecompressor 处理 gzip 压缩
type GzipDecompressor struct{}

// Decompress 实现 gzip 的 Decompressor 接口
func (g *GzipDecompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read gzip data: %w", err)
	}

	return decompressed, nil
}

// BrotliDecompressor 处理 brotli 压缩
type BrotliDecompressor struct{}

// Decompress 实现 brotli 的 Decompressor 接口
func (b *BrotliDecompressor) Decompress(data []byte) ([]byte, error) {
	reader := brotli.NewReader(bytes.NewReader(data))

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read brotli data: %w", err)
	}

	return decompressed, nil
}

// DeflateDecompressor 处理 deflate 压缩（无 gzip 头的原始 DEFLATE 格式）
type DeflateDecompressor struct{}

// Decompress 实现 deflate 的 Decompressor 接口
func (d *DeflateDecompressor) Decompress(data []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(data))
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read deflate data: %w", err)
	}

	return decompressed, nil
}

// ZstdDecompressor 处理 Zstandard 压缩
type ZstdDecompressor struct{}

// Decompress 实现 zstd 的 Decompressor 接口
func (z *ZstdDecompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read zstd data: %w", err)
	}

	return decompressed, nil
}
