package main

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
	Addr    string
	Version string
	SSL     struct {
		CertLocation string
		KeyLocation  string
	}

	mu    sync.RWMutex
	proxy *goproxy.ProxyHttpServer
	srv   *http.Server
}

// Run starts proxy listener.
// Blocking, returns error if failed to start.
func (p *Proxy) Run(ctx context.Context, updatesCh <-chan Rules) error {
	p.updateRules(nil)

	p.srv = &http.Server{
		Addr: p.Addr,
		Handler: rest.Wrap(http.HandlerFunc(p.handle),
			logger.New(logger.Prefix("[DEBUG][proxy]")).Handler,
		),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("[DEBUG] proxy rules listener stopped")
				return
			case rules := <-updatesCh:
				p.updateRules(rules)
			}
		}
	}()

	log.Printf("[INFO] started proxy on %s", p.Addr)
	if err := p.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("start proxy: %w", err)
	}

	return nil
}

func (p *Proxy) updateRules(rules Rules) {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("[DEBUG] updating proxy rules %+v", rules)
	p.proxy = goproxy.NewProxyHttpServer()
	p.proxy.CertStore = certStoreFn(func(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
		cert, err := tls.LoadX509KeyPair(p.SSL.CertLocation, p.SSL.KeyLocation)
		if err != nil {
			return nil, fmt.Errorf("load x509 key pair: %w", err)
		}
		return &cert, nil
	})
	p.proxy.Verbose = true
	p.proxy.KeepHeader = true
	p.proxy.KeepDestinationHeaders = true
	p.proxy.Logger = loggerFn(func(format string, args ...interface{}) {
		log.Printf("[DEBUG][proxy] "+format, args...)
	})

	for _, rule := range rules {
		log.Printf("[DEBUG] adding rule for %s", rule.BaseURL)
		p.proxy.OnRequest(goproxy.UrlMatches(rule.CompiledBaseURL)).HandleConnect(goproxy.AlwaysMitm)

		p.proxy.
			OnRequest(goproxy.ReqConditionFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) bool {
				log.Printf("[DEBUG] checking request for %s", rule.BaseURL)
				return goproxy.UrlMatches(rule.CompiledBaseURL)(req, ctx)
			})).
			DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
				log.Printf("[DEBUG] handling request for %s", rule.BaseURL)
				for k, v := range rule.AddHeaders {
					req.Header.Add(k, v)
				}
				return req, nil
			})

		p.proxy.
			OnResponse(goproxy.RespConditionFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) bool {
				log.Printf("[DEBUG] checking response for %s", rule.BaseURL)
				return goproxy.UrlMatches(rule.CompiledBaseURL)(resp.Request, ctx)
			})).
			DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
				log.Printf("[DEBUG] handling response for %s", rule.BaseURL)
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

	log.Printf("[DEBUG] handling request %s", r.URL)
	p.proxy.ServeHTTP(w, r)
}

type loggerFn func(string, ...interface{})

func (l loggerFn) Printf(format string, v ...interface{}) { l(format, v...) }

type certStoreFn func(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error)

func (f certStoreFn) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	return f(hostname, gen)
}
