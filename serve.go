package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"html/template"
	"io/fs"

	"github.com/alexedwards/scs/v2"
	"github.com/mayowa/templates"
)

type Options struct {
	Host        string
	Port        int
	Public      string
	Middleware  []Middleware
	Routes      []Route
	Log         *slog.Logger
	LogRequests bool
	Templates   *TemplateOptions
	SessionMgr  *scs.SessionManager
}

type TemplateOptions struct {
	Root      string
	Ext       string
	FuncMap   template.FuncMap
	PathToSVG string
	FS        fs.FS
	Debug     bool
}

type Route struct {
	Match   string
	Handler http.Handler
	Name    string
}

type Server struct {
	Host   string
	Port   int
	Public string

	Middleware   []Middleware
	HTTPServer   *http.Server
	routes       []Route
	log          *slog.Logger
	mux          *http.ServeMux
	templateMgr  *templates.Template
	routeMounted bool
	logRequests  bool
	sessionMgr   *scs.SessionManager
	routeNames   map[string]string
}

func Init(option Options) (*Server, error) {
	mux := http.NewServeMux()

	srv := &Server{
		mux:         mux,
		Host:        option.Host,
		Port:        option.Port,
		Public:      option.Public,
		Middleware:  option.Middleware,
		routes:      option.Routes,
		log:         option.Log,
		logRequests: option.LogRequests,
		sessionMgr:  option.SessionMgr,
		routeNames:  make(map[string]string),
	}
	if option.Templates != nil {
		if err := srv.initTemplates(*option.Templates); err != nil {
			return nil, err
		}
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

	return srv, nil
}

func (s *Server) initTemplates(options TemplateOptions) error {
	opts := templates.TemplateOptions{
		Ext:       options.Ext,
		FuncMap:   options.FuncMap,
		PathToSVG: options.PathToSVG,
		FS:        options.FS,
		Debug:     options.Debug,
	}
	tplMgr, err := templates.New(options.Root, &opts)
	if err != nil {
		return err
	}

	s.templateMgr = tplMgr
	return nil
}

// Route mounts the routes to the server. It should be called after all routes are added
// to the server. It is called from Run() if not called before.
func (s *Server) Route() error {
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
		if r.Name != "" {
			s.routeNames[strings.ToLower(r.Name)] = r.Match
		}
	}

	s.mux.Handle("/", chain.Then(root))
	s.routeMounted = true
	return nil
}

type HandleOption struct {
	name       string
	middleware []Middleware
}
type HandleOptionFn func(*HandleOption)

func WithName(name string) HandleOptionFn {
	return func(o *HandleOption) {
		o.name = name
	}
}

func WithMiddleware(middleware ...Middleware) HandleOptionFn {
	return func(o *HandleOption) {
		o.middleware = middleware
	}
}
func (s *Server) Handle(pattern string, handler http.Handler, args ...HandleOptionFn) {
	var options HandleOption
	for _, fn := range args {
		fn(&options)
	}

	if s.routeMounted {
		s.log.Warn("routes already mounted")
		return
	}

	s.routes = append(s.routes, Route{Match: pattern, Handler: handler, Name: options.name})
}

func (s *Server) HandleFunc(pattern string, handler HandlerFunc, args ...HandleOptionFn) {
	s.Handle(pattern, handler, args...)
}

// Group panics if a name isn't provided but named routes are registered
func (s *Server) Group(pattern string, name string, fn func(srv *Server)) {
	grp := http.NewServeMux()
	sub := &Server{}
	fn(sub)

	hasNamedRoutes := false
	for _, r := range sub.routes {
		grp.Handle(r.Match, r.Handler)
		if r.Name != "" {
			rtName := strings.ToLower(fmt.Sprint(name, "/", r.Name))
			s.routeNames[rtName] = path.Join(pattern, r.Match)
			hasNamedRoutes = true
		}
	}

	if hasNamedRoutes && name == "" {
		panic(fmt.Sprintf("group(%q) has named routes but no group name was provided", pattern))
	}

	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}

	mwChain := Chain(sub.Middleware)
	sPattern := pattern[:len(pattern)-1]
	s.Handle(pattern, http.StripPrefix(sPattern, mwChain.Then(grp)))
}

var ErrRoutesNotMounted = errors.New("routes not mounted")

func (s *Server) Run() error {
	if err := s.Route(); err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	slog.Info("listening on", "addr", addr)

	s.HTTPServer.Addr = addr
	return s.HTTPServer.ListenAndServe()
}

type CtxKey string

const (
	CtxKeyServer     CtxKey = "_server_"
	CtxKeySessionMgr CtxKey = "_sessMgr_"
)

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = r.WithContext(context.WithValue(r.Context(), CtxKeyServer, s))
	if s.sessionMgr != nil {
		r = r.WithContext(context.WithValue(r.Context(), CtxKeySessionMgr, s.sessionMgr))
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

// RouteName returns the route path for the given name. If params are provided, they are used to replace
// path parameters in the route path. Path parameters are of the format {param}.
// group route names are prefixed with the group name, separated by a slash.
func (s *Server) RouteName(name string, params ...string) string {
	name = strings.ToLower(name)
	route, found := s.routeNames[name]
	if !found {
		return route
	}

	// path parameters are of the format {param}
	if len(params) > 0 {
		if len(params)%2 != 0 {
			params = append(params, "")
		}

		for i := 0; i < len(params); i += 2 {
			paramKey := "{" + params[i] + "}"
			paramVal := params[i+1]
			route = strings.ReplaceAll(route, paramKey, paramVal)
		}

		return route
	}

	return ""
}
