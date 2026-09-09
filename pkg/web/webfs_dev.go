//go:build !embed

package web

import (
	"io/fs"
	"os"
)

// 默认构建不打包前端产物，运行时从二进制工作目录下的 web/ 目录读取。
// 本地开发请先 `make build-web`，或使用 Vite dev server。

func webFileSystem() fs.FS {
	return os.DirFS("web")
}
