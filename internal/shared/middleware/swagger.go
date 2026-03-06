package middleware

import (
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gnha/gnha-services/internal/shared/config"
	"github.com/labstack/echo/v4"
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
		specs := discoverSpecs("gen/openapi")
		html := buildSwaggerHTML(specs)
		return c.HTML(http.StatusOK, html)
	})
}

// discoverSpecs walks the given directory and returns relative URL paths for all .swagger.json files.
func discoverSpecs(dir string) []string {
	var specs []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ".swagger.json") {
			specs = append(specs, "/swagger/spec/"+strings.TrimPrefix(path, dir+"/"))
		}
		return nil
	})
	return specs
}

// buildSwaggerHTML generates an HTML page with Swagger UI listing all discovered specs.
func buildSwaggerHTML(specs []string) string {
	if len(specs) == 0 {
		return `<!DOCTYPE html><html><body><p>No OpenAPI specs found in gen/openapi/</p></body></html>`
	}
	// Build urls array for SwaggerUI
	var urlEntries string
	for _, s := range specs {
		name := html.EscapeString(strings.TrimSuffix(filepath.Base(s), ".swagger.json"))
		urlEntries += `{url:"` + html.EscapeString(s) + `",name:"` + name + `"},`
	}
	return `<!DOCTYPE html>
<html><head><title>API Docs</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head><body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({urls:[` + urlEntries + `],dom_id:"#swagger-ui","urls.primaryName":"` + html.EscapeString(strings.TrimSuffix(filepath.Base(specs[0]), ".swagger.json")) + `"})</script>
</body></html>`
}
