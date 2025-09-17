# serve

# Server Package

This package provides a lightweight HTTP server framework with support for middleware, templating, and structured logging. It is designed to simplify the process of building web applications by offering a modular and extensible architecture.

## Features

- **Routing**: Define routes with custom handlers using the `Route` struct.
- **Middleware**: Add middleware using the `alice` library for request processing.
- **Templating**: Render HTML templates with the `templates` package.
- **Logging**: Structured logging with customizable log levels and formats.
- **Session Management**: Optional session management using `scs`.

## Key Components

### `ServerMux`
The main server struct that manages routes, middleware, and server configuration.

- **Initialization**: Use `Init(options Options)` to create a new server instance.
- **Routing**: use `Handle`,  `HandleFunc`, or `Group` to add routes then call `Route()` to set up routes and middleware. 
Calling `Route()` is optional as it will be called automatically when `Run()` is called.
- **Running**: Start the server with `Run()`.

### `Context`
Provides utilities for handling requests and responses.

- `Request()`: Access the HTTP request.
- `Response()`: Access the HTTP response writer.
- `Render(status int, ctx RenderOpt)`: Render an HTML template.
- `String(code int, out string)`: Send a plain text response.
- `Log()`: Access a scoped logger.
- `Session()`: Access the session manager.
- `Redirect(statusCode int, url string)`: Redirect the client to a new URL.

### Middleware
Predefined middleware for common tasks:
- `RequestIDMiddleware`: Adds a unique request ID to each request.
- `RecoveryMiddleware`: Recovers from panics and logs errors.

### Logging
Customizable logging with support for JSON and text formats. Use `InitLog` to configure logging behavior.

### Templates
Initialize templates with `InitTemplates` to render HTML views.

## Example Usage

```go
package main

import (
	"net/http"
	"context"
	"log/slog"
	"os"
	"fmt"
	
	"github.com/actanonv/server"
)

func main() {
	options := server.Options{
		Host: "localhost",
		Port: 8080,
		Middleware: []server.Middleware{
			server.RequestIDMiddleware,
			server.RecoveryMiddleware,
		},
	}

	srv := server.Init(options)
	srv.HandleFunc("/", func(ctx server.Context) error {
		return ctx.String(http.StatusOK, "Hello, World!")
	})

	srv.Group("/greet", func(srv *server.Server) {
		srv.Middleware = []server.Middleware{
			func(next http.Handler) http.Handler {
				return server.HandlerFunc(func(ctx server.Context) error {
					r := ctx.Request()
					r = r.WithContext(context.WithValue(r.Context(), "age", 22))

					next.ServeHTTP(ctx.Response(), r)
					return nil
				})
			},
		}

		srv.HandleFunc("/hello", func(ctx server.Context) error {
			age := ctx.Request().Context().Value("age")
			return ctx.String(http.StatusOK, fmt.Sprint("Hello, ", age ,"year old Grouped World!"))
		})
		srv.HandleFunc("/goodbye", func(ctx server.Context) error {
			return ctx.String(http.StatusOK, "Goodbye, Grouped World!")
		})
	})

	if err := srv.Run(); err != nil {
		slog.Error("Server run failed", "error", err)
		os.Exit(1)
	}
}
```

For more details, refer to the source code and comments.