package static

import (
	"io/fs"
	"mime"
	"path"
	"strings"

	"github.com/valyala/fasthttp"
)

const assetsRoot = "assets"

func assetContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	}
	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func cleanAssetName(value any) (string, bool) {
	raw, ok := value.(string)
	if !ok {
		return "", false
	}
	name := strings.TrimPrefix(strings.ReplaceAll(raw, "\\", "/"), "/")
	if name == "" || strings.Contains(name, "\x00") {
		return "", false
	}
	cleaned := path.Clean(name)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", false
	}
	return cleaned, true
}

func HandleAsset(ctx *fasthttp.RequestCtx) {
	name, ok := cleanAssetName(ctx.UserValue("filepath"))
	if !ok {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	assetPath := path.Join(assetsRoot, name)
	info, err := fs.Stat(AssetsFS, assetPath)
	if err != nil || info.IsDir() {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	ctx.SetContentType(assetContentType(assetPath))
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.SetStatusCode(fasthttp.StatusOK)
	if ctx.IsHead() {
		ctx.Response.Header.SetContentLength(int(info.Size()))
		return
	}
	body, err := AssetsFS.ReadFile(assetPath)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		return
	}
	ctx.SetBody(body)
}
