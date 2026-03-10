package middleware

import (
	"html"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnha/golang-echo-boilerplate/internal/shared/config"
	"github.com/labstack/echo/v4"
)

// Pin swagger-ui-dist to a specific version for supply-chain safety.
// Avoid using floating tags like "@5" which can resolve to arbitrary versions.
const (
	swaggerUIVersion = "5.18.2"
	swaggerCSSURL    = "https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/" + swaggerUIVersion + "/swagger-ui.css"
	swaggerJSURL     = "https://cdnjs.cloudflare.com/ajax/libs/swagger-ui/" + swaggerUIVersion + "/swagger-ui-bundle.js"
)

// MountSwagger serves OpenAPI specs and Swagger UI in non-production environments.
func MountSwagger(e *echo.Echo, cfg *config.Config) {
	if cfg.AppEnv == "production" {
		return
	}

	// Serve OpenAPI specs from gen/openapi/
	e.Static("/swagger/spec", "gen/openapi")

	// Swagger UI via CDN redirect — auto-discovers all .swagger.json specs
	e.GET("/swagger", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/swagger/")
	})
	e.GET("/swagger/", func(c echo.Context) error {
		// Override the global CSP to allow Swagger UI CDN assets.
		c.Response().Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' https://cdnjs.cloudflare.com 'unsafe-inline'; "+
				"script-src 'self' https://cdnjs.cloudflare.com 'unsafe-inline'; "+
				"img-src 'self' data:")
		specs := discoverSpecs("gen/openapi")
		swaggerHTML := buildSwaggerHTML(specs)
		return c.HTML(http.StatusOK, swaggerHTML)
	})
}

// discoverSpecs walks the given directory and returns relative URL paths for all .swagger.json files.
func discoverSpecs(dir string) []string {
	var specs []string
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			slog.Warn("swagger spec discovery error", "path", path, "err", err)
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".swagger.json") {
			specs = append(specs, "/swagger/spec/"+strings.TrimPrefix(path, dir+"/"))
		}
		return nil
	}); err != nil {
		slog.Warn("swagger spec walk failed", "dir", dir, "err", err)
	}
	return specs
}

// buildSwaggerHTML generates an HTML page with Swagger UI listing all discovered specs.
func buildSwaggerHTML(specs []string) string {
	if len(specs) == 0 {
		return `<!DOCTYPE html><html><body><p>No OpenAPI specs found in gen/openapi/</p></body></html>`
	}
	return `<!DOCTYPE html>
<html><head><title>API Docs</title>
<link rel="stylesheet" href="` + swaggerCSSURL + `">
</head><body>
<div id="swagger-ui"></div>
<script src="` + swaggerJSURL + `"></script>
<script>SwaggerUIBundle({url:"` + html.EscapeString(specs[0]) + `",dom_id:"#swagger-ui"})</script>
</body></html>`
}
