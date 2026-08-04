package http

import (
	"net/http"

	"github.com/jcamilovallejos/linkguard/api"
)

// swaggerUIPage loads Swagger UI from a CDN and points it at the service's
// own embedded OpenAPI document, so the two endpoints can be tried out
// from a browser without Postman or any code generation step.
const swaggerUIPage = `<!DOCTYPE html>
<html>
<head>
  <title>linkguard API docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => SwaggerUIBundle({ url: "/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`

func handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(api.Spec)
}

func handleSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIPage))
}
