//go:build embed

package web

import (
	"embed"
	"io/fs"
)

// 使用 `-tags embed` 构建时，将前端构建产物打包进二进制，
// 实现单文件部署。产物由 `make build-web` 生成到 pkg/web/dist。
//
//go:embed all:dist
var embeddedDistFS embed.FS

func webFileSystem() fs.FS {
	sub, err := fs.Sub(embeddedDistFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
