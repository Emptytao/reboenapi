package router

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the embedded frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS      embed.FS
	DefaultIndexPage    []byte
	ClassicBuildFS      embed.FS
	ClassicIndexPage    []byte
	PlaygroundBuildFS   embed.FS
	PlaygroundIndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")
	themeFS := common.NewThemeAwareFS(defaultFS, classicFS)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/", themeFS))
	registerPlaygroundRoutes(router, assets.PlaygroundBuildFS, assets.PlaygroundIndexPage)
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		if common.GetTheme() == "classic" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}

func registerPlaygroundRoutes(router *gin.Engine, buildFS embed.FS, indexPage []byte) {
	if len(indexPage) == 0 {
		return
	}

	subFS, err := fs.Sub(buildFS, "playground/dist")
	if err != nil {
		panic(err)
	}
	httpFS := http.FS(subFS)

	serveIndex := func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	}

	serveAsset := func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		filePath := strings.TrimPrefix(c.Param("filepath"), "/")
		if filePath == "" {
			serveIndex(c)
			return
		}

		file, err := httpFS.Open(filePath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil || stat.IsDir() {
			c.Status(http.StatusNotFound)
			return
		}

		c.FileFromFS(filePath, httpFS)
	}

	router.GET("/playground", func(c *gin.Context) {
		target := "/playground/"
		if rawQuery := strings.TrimSpace(c.Request.URL.RawQuery); rawQuery != "" {
			target = fmt.Sprintf("%s?%s", target, rawQuery)
		}
		c.Redirect(http.StatusTemporaryRedirect, target)
	})
	router.GET("/playground/", serveIndex)
	router.GET("/playground/*filepath", serveAsset)
}
