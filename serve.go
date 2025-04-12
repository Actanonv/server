package server

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
	Log         *slog.Logger
	LogRequests bool
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
	log         *slog.Logger

	Mux          *http.ServeMux
	templates    *templates.Template
	chain        *alice.Chain
	routeMounted bool
	logRequests  bool
}

func Init(option Options) *ServerMux {
	mux := http.NewServeMux()

	srv := &ServerMux{
		Mux:         mux,
		Host:        option.Host,
		Port:        option.Port,
		Public:      option.Public,
		Middlewares: option.Middlewares,
		Routes:      option.Routes,
		log:         option.Log,
		logRequests: option.LogRequests,
		templates:   option.Templates,
	}

	if srv.log == nil {
		srv.log = appLog
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

func (s *ServerMux) Run() error {
	if !s.routeMounted {
		return ErrRoutesNotMounted
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	slog.Info("listening on", "addr", addr)
	return http.ListenAndServe(addr, s)
}

func (s *ServerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.logRequests {
		s.Mux.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	rw := &ResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.Mux.ServeHTTP(rw, r)
	s.log.Info(r.RequestURI, "method", r.Method, "path", r.URL.Path, "status", rw.statusCode, "duration", time.Since(start))

}
