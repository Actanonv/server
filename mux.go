package mux

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/justinas/alice"
	"github.com/mayowa/templates"
)

type Options struct {
	Host        string
	Port        int
	Public      string
	Middlewares []alice.Constructor
	Routes      []Route
	Logging     bool
	Templates   *templates.Template
}

type Route struct {
	Match     string
	HandlerFn HandlerFunc
}

type ServerMux struct {
	Host   string
	Port   int
	Public string

	Middlewares []alice.Constructor
	Routes      []Route

	Mux          *http.ServeMux
	templates    *templates.Template
	chain        *alice.Chain
	routeMounted bool
	withLogging  bool
}

func Server(option Options) *ServerMux {
	mux := http.NewServeMux()

	srv := &ServerMux{
		Mux:         mux,
		Host:        option.Host,
		Port:        option.Port,
		Public:      option.Public,
		Middlewares: option.Middlewares,
		Routes:      option.Routes,
		withLogging: option.Logging,
		templates:   option.Templates,
	}

	return srv
}

func (s *ServerMux) Route() error {
	chain := alice.New(s.Middlewares...)
	s.chain = &chain

	pubFolder := s.Public
	if pubFolder == "" {
		pubFolder = "./public"
	}

	s.Mux.Handle("/public/", s.chain.Then(http.StripPrefix("/public", http.FileServer(http.Dir(pubFolder)))))

	for _, r := range s.Routes {
		s.Mux.Handle(r.Match, chain.Then(r.HandlerFn))
	}

	s.routeMounted = true
	return nil
}

var ErrRoutesNotMounted = errors.New("routes not mounted")

func (s *ServerMux) Serve() error {
	if !s.routeMounted {
		return ErrRoutesNotMounted
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	slog.Info("listening on", "addr", addr)
	return http.ListenAndServe(addr, s)
}

func (s *ServerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.withLogging {
		s.Mux.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	rw := &ResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.Mux.ServeHTTP(rw, r)
	slog.Info(r.RequestURI, "method", r.Method, "path", r.URL.Path, "status", rw.statusCode, "duration", time.Since(start))

}
