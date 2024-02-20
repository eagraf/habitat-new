package reverse_proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

type RuleSet map[string]Rule

const ReverseProxyPort = "3001"

type ProxyServer struct {
	logger *zerolog.Logger
	server *http.Server
	Rules  RuleSet
}

func NewProxyServer(lc fx.Lifecycle, logger *zerolog.Logger) (*ProxyServer, RuleSet) {
	srv := &ProxyServer{
		logger: logger,
		Rules:  make(RuleSet),
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listenAddr := fmt.Sprintf(":%s", ReverseProxyPort)
			logger.Info().Msgf("Starting Habitat reverse proxy server at %s", listenAddr)
			go srv.Start(listenAddr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.server.Shutdown(ctx)
		},
	})
	return srv, srv.Rules
}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, rule := range s.Rules {
		if rule.Match(r.URL) {
			rule.Handler().ServeHTTP(w, r)
			return
		}
	}
	// No rules matched
	w.WriteHeader(http.StatusNotFound)
}

func (s *ProxyServer) Start(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Msg("reverse proxy server failed to start")
	}
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s,
	}
	s.server = httpServer
	log.Fatal().Err(httpServer.Serve(ln)).Msg("reverse proxy server failed")
}

func (r RuleSet) Add(name string, rule Rule) error {
	if _, ok := r[name]; ok {
		return fmt.Errorf("rule name %s is already taken", name)
	}
	r[name] = rule
	return nil
}

func (r RuleSet) Remove(name string) error {
	if _, ok := r[name]; !ok {
		return fmt.Errorf("rule %s does not exist", name)
	}
	delete(r, name)
	return nil
}

type Rule interface {
	Match(url *url.URL) bool
	Handler() http.Handler
}

type FileServerRule struct {
	Matcher string
	Path    string
}

func (r *FileServerRule) Match(url *url.URL) bool {
	// TODO make this work with actual glob strings
	// For now, just match based off of base path
	return strings.HasPrefix(url.Path, r.Matcher)
}

func (r *FileServerRule) Handler() http.Handler {
	return &FileServerHandler{
		Prefix: r.Matcher,
		Path:   r.Path,
	}
}

type FileServerHandler struct {
	Prefix string
	Path   string
}

func (h *FileServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to remove prefix
	oldPath := r.URL.Path
	r.URL.Path = strings.TrimPrefix(oldPath, h.Prefix)

	if oldPath == r.URL.Path {
		// Something weird happened
		_, _ = w.Write([]byte("unable to remove url path prefix"))
		w.WriteHeader(http.StatusInternalServerError)
	}

	http.FileServer(http.Dir(h.Path)).ServeHTTP(w, r)
}

type RedirectRule struct {
	Matcher         string
	ForwardLocation *url.URL
}

func (r *RedirectRule) Match(url *url.URL) bool {
	// TODO make this work with actual glob strings
	// For now, just match based off of base path
	return strings.HasPrefix(url.Path, r.Matcher)
}

func (r *RedirectRule) Handler() http.Handler {
	target := r.ForwardLocation.Host

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = r.ForwardLocation.Scheme
			req.URL.Host = target
			req.URL.Path = strings.TrimPrefix(req.URL.Path, r.Matcher) // TODO this needs to be fixed when globs are implemented
		},
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
		},
		ModifyResponse: func(res *http.Response) error {
			return nil
		},
		ErrorHandler: func(rw http.ResponseWriter, r *http.Request, err error) {
			log.Error().Err(err).Msg("reverse proxy request forwarding error")
			_, _ = rw.Write([]byte(err.Error()))
			rw.WriteHeader(http.StatusInternalServerError)
		},
	}
}

func GetRuleFromConfig(config *ProxyRule, appPath string) (Rule, error) {
	switch config.Type {
	// TODO apps need to hook into the fileserver so that the necessary files are loaded into the
	// container when the application is active.
	case ProxyRuleFileServer:
		return &FileServerRule{
			Matcher: config.Matcher,
			Path:    filepath.Join(appPath, config.Target),
		}, nil
	case ProxyRuleRedirect:
		targetURL, err := url.Parse(config.Target)
		if err != nil {
			return nil, fmt.Errorf("error parsing url for RedirectRule: %s", err)
		}
		return &RedirectRule{
			Matcher:         config.Matcher,
			ForwardLocation: targetURL,
		}, nil
	default:
		return nil, fmt.Errorf("no proxy rule type %s", config.Type)
	}
}
