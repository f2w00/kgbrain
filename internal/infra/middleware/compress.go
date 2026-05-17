package middleware

import (
	"compress/gzip"
	"compress/zlib"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

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
