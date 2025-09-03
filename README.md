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

### `Route`
Defines a route with a URL pattern and a handler function.

```go
type Route struct {
    Match     string
    Handler   http.Handler
}
```

### `HandlerFunc`
Custom handler type that operates on a `Context` object.

```go
type HandlerFunc func(Context) error
```

### `Context`
Provides utilities for handling requests and responses.

- `Request()`: Access the HTTP request.
- `Response()`: Access the HTTP response writer.
- `Render(status int, ctx RenderOpt)`: Render an HTML template.
- `String(code int, out string)`: Send a plain text response.
- `Log()`: Access a scoped logger.

### Middleware
Predefined middleware for common tasks:
- `RemoveTrailingSlashMiddleware`: Removes trailing slashes from URLs.
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
    "github.com/actanonv/server"
    "github.com/justinas/alice"
)

func main() {
    options := server.Options{
        Host: "localhost",
        Port: 8080,
        Routes: []server.Route{
            {
                Match: "/",
                Handler: server.HandlerFunc(func (ctx server.Context) error {
                    return ctx.String(200, "Hello, World!")
                }),
            },
        },
        Middlewares: []alice.Constructor{
            server.RemoveTrailingSlashMiddleware,
            server.RequestIDMiddleware,
            server.RecoveryMiddleware,
        },
    }

    srv := server.Init(options)
    srv.Route()
    srv.Run()
}
```

For more details, refer to the source code and comments.