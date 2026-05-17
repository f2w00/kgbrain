package middleware

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// compressWriter 包装 ResponseWriter, 将写入的响应体通过压缩器输出
type compressWriter struct {
	http.ResponseWriter
	writer io.WriteCloser
}

func (w *compressWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// DecompressBody 解压请求体中间件, 支持 gzip / deflate / zstd
// 按 Content-Encoding 头自动选择解压算法, 解压后替换 r.Body
func DecompressBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enc := strings.TrimSpace(strings.ToLower(r.Header.Get("Content-Encoding")))
		if enc == "" {
			next.ServeHTTP(w, r)
			return
		}

		var reader io.ReadCloser
		var err error

		switch enc {
		case "gzip":
			reader, err = gzip.NewReader(r.Body)
		case "deflate":
			reader, err = zlib.NewReader(r.Body)
		case "zstd":
			// zstd.Decoder 不实现 io.ReadCloser, 用 NopCloser 包装
			var dec *zstd.Decoder
			dec, err = zstd.NewReader(r.Body)
			if err == nil {
				reader = io.NopCloser(dec)
			}
		default:
			http.Error(w, "unsupported Content-Encoding: "+enc, http.StatusUnsupportedMediaType)
			return
		}

		if err != nil {
			http.Error(w, "invalid "+enc+" body", http.StatusBadRequest)
			return
		}
		defer reader.Close()

		// 替换请求体, 清除 Content-Encoding 头, 重置 ContentLength
		r.Body = reader
		r.Header.Del("Content-Encoding")
		r.ContentLength = -1
		next.ServeHTTP(w, r)
	})
}

// CompressResponse 压缩响应体中间件, 按 Accept-Encoding 协商 (zstd > gzip > 无)
// 优先 zstd, 其次 gzip, 都不支持则不压缩
func CompressResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ae := r.Header.Get("Accept-Encoding")
		if ae == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 优先 zstd
		if strings.Contains(ae, "zstd") {
			zw, err := zstd.NewWriter(w)
			if err != nil {
				http.Error(w, "compress error", http.StatusInternalServerError)
				return
			}
			defer zw.Close()
			w.Header().Set("Content-Encoding", "zstd")
			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(&compressWriter{ResponseWriter: w, writer: zw}, r)
			return
		}

		// 其次 gzip
		if strings.Contains(ae, "gzip") {
			gw := gzip.NewWriter(w)
			defer gw.Close()
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			next.ServeHTTP(&compressWriter{ResponseWriter: w, writer: gw}, r)
			return
		}

		// 不支持压缩, 直接透传
		next.ServeHTTP(w, r)
	})
}