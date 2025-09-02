package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/mayowa/templates"
)

type MuxOptions struct {
	Host        string
	Port        int
	Public      string
	Middlewares []Middleware
	Routes      []Route
	Log         *slog.Logger
	LogRequests bool
	Templates   *templates.Template
	SessionMgr  *scs.SessionManager
}

type Route struct {
	Match   string
	Handler http.Handler
}

type ServerMux struct {
	Host   string
	Port   int
	Public string

	Middleware   []Middleware
	HTTPServer   *http.Server
	routes       []Route
	log          *slog.Logger
	mux          *http.ServeMux
	templates    *templates.Template
	routeMounted bool
	logRequests  bool
	sessionMgr   *scs.SessionManager
}

func Init(option MuxOptions) *ServerMux {
	mux := http.NewServeMux()

	srv := &ServerMux{
		mux:         mux,
		Host:        option.Host,
		Port:        option.Port,
		Public:      option.Public,
		Middleware:  option.Middlewares,
		routes:      option.Routes,
		log:         option.Log,
		logRequests: option.LogRequests,
		templates:   option.Templates,
		sessionMgr:  option.SessionMgr,
	}

	if srv.log == nil {
		srv.log = appLog
	}

	srv.HTTPServer = &http.Server{}

	var s http.Handler = srv
	if srv.sessionMgr != nil {
		s = srv.sessionMgr.LoadAndSave(s)
	}
	srv.HTTPServer.Handler = s

	return srv
}

// Route mounts the routes to the server. It should be called after all routes are added
// to the server. It is called from Run() if not called before.
func (s *ServerMux) Route() error {
	if s.routeMounted {
		return nil
	}

	chain := Chain(s.Middleware)
	pubFolder := s.Public
	if pubFolder == "" {
		pubFolder = "./public"
	}

	s.mux.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.Dir(pubFolder))))
	root := http.NewServeMux()
	for _, r := range s.routes {
		root.Handle(r.Match, r.Handler)
	}

	s.mux.Handle("/", chain.Then(root))
	s.routeMounted = true
	return nil
}

func (s *ServerMux) Handle(pattern string, handler http.Handler) {
	if s.routeMounted {
		s.log.Warn("routes already mounted")
		return
	}

	s.routes = append(s.routes, Route{pattern, handler})
}

func (s *ServerMux) HandleFunc(pattern string, handler HandlerFunc) {
	s.Handle(pattern, handler)
}

func (s *ServerMux) Group(pattern string, fn func(srv *ServerMux)) {
	grp := http.NewServeMux()
	sub := &ServerMux{}
	fn(sub)

	for _, r := range sub.routes {
		grp.Handle(r.Match, r.Handler)
	}
	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}

	mwChain := Chain(sub.Middleware)
	sPattern := pattern[:len(pattern)-1]
	s.Handle(pattern, http.StripPrefix(sPattern, mwChain.Then(grp)))
}

var ErrRoutesNotMounted = errors.New("routes not mounted")

func (s *ServerMux) Run() error {
	if err := s.Route(); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	slog.Info("listening on", "addr", addr)

	s.HTTPServer.Addr = addr
	return s.HTTPServer.ListenAndServe()
}

func (s *ServerMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.sessionMgr != nil {
		r = r.WithContext(context.WithValue(r.Context(), "_sessMgr_", s.sessionMgr))
	}

	if !s.logRequests {
		s.mux.ServeHTTP(w, r)
		return
	}

	start := time.Now()
	rw := &ResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	s.mux.ServeHTTP(rw, r)
	s.log.Info(r.RequestURI, "method", r.Method, "path", r.URL.Path, "status", rw.statusCode, "duration", time.Since(start))

}
