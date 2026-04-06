package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"html/template"
	"io/fs"

	"github.com/alexedwards/scs/v2"
)

type ErrorFunc func(ctx Context, err error)

type Options struct {
	Host        string
	Port        int
	Public      string
	Middleware  []Middleware
	Routes      []Route
	Log         *slog.Logger
	LogRequests bool
	SessionMgr  *scs.SessionManager
	ErrorFunc   ErrorFunc
	Debug       bool
	DisableLoadAndSave bool
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
	routeMounted bool
	logRequests  bool
	sessionMgr   *scs.SessionManager
	routeNames   map[string]string
	errorFunc    ErrorFunc
	debug        bool

	mu               sync.RWMutex
	routeNamesAtomic atomic.Value // stores map[string]string
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
		errorFunc:   option.ErrorFunc,
		debug:       option.Debug,
	}
	srv.routeNamesAtomic.Store(make(map[string]string))

	if srv.log == nil {
		srv.log = appLog
	}

	srv.HTTPServer = &http.Server{}

	var s http.Handler = srv
	if srv.sessionMgr != nil && !option.DisableLoadAndSave {
		s = srv.sessionMgr.LoadAndSave(s)
	}
	srv.HTTPServer.Handler = s

	return srv, nil
}

// Route mounts the routes to the server. It should be called after all routes are added
// to the server. It is called from Run() if not called before.
func (s *Server) Route() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.routeMounted {
		return nil
	}

	chain := Chain(s.Middleware)
	pubFolder := s.Public
	if pubFolder == "" {
		pubFolder = "./public"
	}

	s.mux.Handle("/public/", http.StripPrefix("/public", http.FileServer(NeuteredFileSystem{http.Dir(pubFolder)})))
	root := http.NewServeMux()
	for _, r := range s.routes {
		root.Handle(r.Match, r.Handler)
		if r.Name != "" {
			s.addRouteNameLocked(r.Name, r.Match)
		}
	}

	s.mux.Handle("/", chain.Then(root))
	s.routeMounted = true

	// Update atomic routeNames for lock-free reads
	s.updateRouteNamesAtomicLocked()

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

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.routeMounted {
		s.log.Warn("routes already mounted")
		return
	}

	if len(options.middleware) > 0 {
		handler = Chain(options.middleware).Then(handler)
	}

	s.routes = append(s.routes, Route{Match: pattern, Handler: handler, Name: options.name})

	if options.name != "" {
		s.addRouteNameLocked(options.name, pattern)
		s.updateRouteNamesAtomicLocked()
	}
}

func (s *Server) HandleFunc(pattern string, handler HandlerFunc, args ...HandleOptionFn) {
	s.Handle(pattern, handler, args...)
}

// Group panics if a name isn't provided but named routes are registered
func (s *Server) Group(pattern string, name string, fn func(srv *Server)) {
	grp := http.NewServeMux()
	sub := &Server{
		log:        s.log,
		sessionMgr: s.sessionMgr,
		routeNames: make(map[string]string),
	}
	fn(sub)

	s.mu.Lock()
	defer s.mu.Unlock()

	hasNamedRoutes := false
	for _, r := range sub.routes {
		grp.Handle(r.Match, r.Handler)
		if r.Name != "" {
			s.addRouteNameLocked(fmt.Sprint(name, "/", r.Name), path.Join(pattern, r.Match))
			hasNamedRoutes = true
		}
	}

	if hasNamedRoutes && name == "" {
		panic(fmt.Sprintf("group(%q) has named routes but no group name was provided", pattern))
	}

	// Update atomic routeNames if new names were added
	if hasNamedRoutes {
		s.updateRouteNamesAtomicLocked()
	}

	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}

	mwChain := Chain(sub.Middleware)
	_, _, sPath := PatternParts(pattern)
	if sPath == "" {
		sPath = pattern
	}

	if strings.HasSuffix(sPath, "/") && len(sPath) > 1 {
		sPath = sPath[:len(sPath)-1]
	}

	// s.Handle already takes the lock, but we are inside a locked block here.
	// We need a way to add routes without re-locking.
	// For now, let's unlock before calling Handle or implement a HandleLocked.
	// However, Handle calls append to s.routes which we need to protect.

	s.routes = append(s.routes, Route{
		Match:   pattern,
		Handler: http.StripPrefix(sPath, mwChain.Then(grp)),
		Name:    "",
	})
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
	routeMap, ok := s.routeNamesAtomic.Load().(map[string]string)
	if !ok {
		return ""
	}

	route, found := routeMap[name]
	if !found {
		return route
	}

	// path parameters are of the format {param}
	if len(params) > 0 {
		if len(params)%2 != 0 {
			params = append(params, "")
		}

		pairs := make([]string, 0, len(params))
		for i := 0; i < len(params); i += 2 {
			pairs = append(pairs, "{"+params[i]+"}", params[i+1])
		}
		replacer := strings.NewReplacer(pairs...)
		return replacer.Replace(route)
	}

	return route
}

func (s *Server) updateRouteNamesAtomicLocked() {
	newMap := make(map[string]string, len(s.routeNames))
	for k, v := range s.routeNames {
		newMap[k] = v
	}
	s.routeNamesAtomic.Store(newMap)
}

func (s *Server) addRouteName(name string, pattern string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addRouteNameLocked(name, pattern)
}

func (s *Server) addRouteNameLocked(name string, pattern string) {
	_, host, pth := PatternParts(pattern)
	if host == "" && pth == "" {
		s.log.Warn("route name not added", "name", name, "pattern", pattern)
		return
	}

	s.routeNames[strings.ToLower(name)] = host + pth
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.HTTPServer.Shutdown(ctx)
}

// NeuteredFileSystem prevents directory listing
type NeuteredFileSystem struct {
	fs http.FileSystem
}

func (nfs NeuteredFileSystem) Open(path string) (http.File, error) {
	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if s.IsDir() {
		index := strings.TrimSuffix(path, "/") + "/index.html"
		if _, err := nfs.fs.Open(index); err != nil {
			f.Close()
			return nil, err
		}
	}

	return f, nil
}

var rePattern = regexp.MustCompile(`^(?:(\w+)\s+)?([^/ ]+)?(/.*)?$`)

func PatternParts(pattern string) (method, name, path string) {
	parts := rePattern.FindStringSubmatch(pattern)
	if len(parts) != 4 {
		return "", "", ""
	}

	return parts[1], parts[2], parts[3]
}
