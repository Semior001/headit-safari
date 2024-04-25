package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/elazarl/goproxy"
	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"
)

// Proxy proxies requests.
type Proxy struct {
	Addr        string
	Version     string
	Certificate tls.Certificate
	FallbackURL string

	mu    sync.RWMutex
	rules Rules
	proxy *goproxy.ProxyHttpServer
	srv   *http.Server
}

// Run starts proxy server.
// Blocking, returns error if failed to start.
func (p *Proxy) Run() error {
	p.mu.Lock()

	p.srv = &http.Server{
		Addr: p.Addr,
		Handler: rest.Wrap(http.HandlerFunc(p.handle),
			logger.New(logger.Prefix("[DEBUG][proxy]")).Handler,
		),
	}

	log.Printf("[INFO] started proxy on %s", p.Addr)
	if err := p.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("start proxy: %w", err)
	}

	return nil
}

// Shutdown stops proxy server.
func (p *Proxy) Shutdown(ctx context.Context) error {
	if err := p.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http serveer: %w", err)
	}
	return nil
}

// UpdateRules updates proxy rules.
func (p *Proxy) UpdateRules(rules Rules) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.rules = rules

	log.Printf("[DEBUG] updating proxy rules %+v", rules)
	p.updateProxy(rules)
}

func (p *Proxy) updateProxy(rules Rules) {
	p.proxy = goproxy.NewProxyHttpServer()
	p.proxy.CertStore = staticCert(p.Certificate)
	p.proxy.Verbose = true
	p.proxy.KeepHeader = true
	p.proxy.KeepDestinationHeaders = true
	p.proxy.Logger = loggerFn(func(format string, args ...interface{}) {
		log.Printf("[DEBUG][proxy] "+format, args...)
	})

	for _, rule := range rules {
		log.Printf("[DEBUG] adding rule for %s", rule.BaseURL)
		p.proxy.OnRequest(goproxy.UrlMatches(rule.CompiledBaseURL)).HandleConnect(goproxy.AlwaysMitm)
		p.proxy.OnRequest(goproxy.UrlMatches(rule.CompiledBaseURL)).
			DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
				for k, v := range rule.AddHeaders {
					req.Header.Add(k, v)
				}
				return req, nil
			})

		p.proxy.OnResponse(goproxy.UrlMatches(rule.CompiledBaseURL)).
			DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
				resp.Header.Add("X-Proxied-By", fmt.Sprintf("headit-safari-proxy %v", p.Version))
				for k, v := range rule.AddHeaders {
					resp.Header.Add("X-Added-Header-"+k, v)
				}
				return resp
			})
	}
}

func (p *Proxy) handle(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// catch only URLs that are in rules
	for _, rule := range p.rules {
		if rule.CompiledBaseURL.MatchString(r.URL.Host) {
			p.proxy.ServeHTTP(w, r)
			return
		}
	}

	// fallback to FallbackURL
	if p.FallbackURL != "" {
		http.Redirect(w, r, p.FallbackURL, http.StatusTemporaryRedirect)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}
