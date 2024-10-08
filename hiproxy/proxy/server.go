package proxy

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/AdguardTeam/gomitmproxy"
	"github.com/AdguardTeam/gomitmproxy/mitm"
	"github.com/go-chi/cors"
	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"
)

// Server proxies requests.
type Server struct {
	APIPort     int
	ProxyPort   int
	Version     string
	Certificate tls.Certificate
	Insecure    bool

	mu     sync.RWMutex
	proxy  *gomitmproxy.Proxy
	apiSrv *http.Server
	rules  Rules
}

// Run starts proxy server, non-blocking.
func (p *Server) Run() error {
	p.mu.Lock()
	privateKey := p.Certificate.PrivateKey.(*rsa.PrivateKey)
	x509c, err := x509.ParseCertificate(p.Certificate.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse certificate: %w", err)
	}

	mitmCfg, err := mitm.NewConfig(x509c, privateKey, nil)
	if err != nil {
		return fmt.Errorf("make mitm config: %w", err)
	}

	if p.Insecure {
		log.Printf("[warn] running in insecure mode, minimum for local TLS version is set to 1.0")
		mitmCfg.SetTLSConfig(&tls.Config{
			MinVersion: tls.VersionTLS10,
			NextProtos: []string{"http/1.1"},
			// Accept client certs without verifying them
			// Note that we will still verify remote server certs
			// nolint:gosec
			InsecureSkipVerify: true,
		})
	}

	cfg := gomitmproxy.Config{
		ListenAddr:     &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p.ProxyPort}, // listen localhost only
		MITMConfig:     mitmCfg,
		MITMExceptions: []string{"localhost"},
		APIHost:        fmt.Sprintf("localhost:%d", p.ProxyPort),
		OnRequest:      p.appendHeadersToRequest,
		OnResponse:     p.appendHeadersToResponse,
		OnError: func(se *gomitmproxy.Session, err error) {
			log.Printf("[warn][mitm][%s|%s] proxy error: %v", se.ID(), se.Request().URL, err)
		},
	}

	p.proxy = gomitmproxy.NewProxy(cfg)
	p.apiSrv = &http.Server{
		Addr: fmt.Sprintf("localhost:%d", p.APIPort),
		Handler: rest.Wrap(http.HandlerFunc(p.handleRulesRequest),
			logger.New(logger.Prefix("[info][API]"), logger.WithBody).Handler,
			cors.AllowAll().Handler,
		),
	}
	p.mu.Unlock()

	go func() {
		log.Printf("[info] starting API server on %s", p.apiSrv.Addr)
		if err := p.apiSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[error] failed to start API server: %v", err)
		}
	}()

	if err = p.proxy.Start(); err != nil {
		return fmt.Errorf("run proxy: %w", err)
	}

	return nil
}

func (p *Server) appendHeadersToRequest(se *gomitmproxy.Session) (*http.Request, *http.Response) {
	log.Printf("[debug][mitm][%s|%s] request: %s", se.ID(), se.Request().URL, se.Request().Method)

	p.mu.RLock()
	defer p.mu.RUnlock()

	rule, found := Rule{}, false
	for _, r := range p.rules {
		if r.Host == se.Request().Host {
			rule, found = r, true
			break
		}
	}

	if !found {
		return nil, nil
	}

	se.Ctx().SetProp("rule", rule)

	for k, v := range rule.AddHeaders {
		se.Request().Header.Set(k, v)
	}

	return se.Request(), nil
}

func (p *Server) appendHeadersToResponse(se *gomitmproxy.Session) *http.Response {
	iface, ok := se.Ctx().GetProp("rule")
	if !ok {
		return nil
	}

	rule, ok := iface.(Rule)
	if !ok {
		panic(fmt.Errorf("rule was expected to be of type Rule, got %T", rule))
	}

	se.Response().Header.Set("X-Proxied-By", fmt.Sprintf("headit %s", p.Version))

	for k, v := range rule.AddHeaders {
		se.Response().Header.Set(fmt.Sprintf("X-Added-Header-%s", k), v)
	}
	return se.Response()
}

// Close stops the proxy server.
func (p *Server) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.apiSrv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown API server: %w", err)
	}

	// we don't want to wait for all connections to be properly closed,
	// otherwise that's just frustrating
	proxyClosed := make(chan struct{})
	go func() {
		p.proxy.Close()
		close(proxyClosed)
	}()

	select {
	case <-ctx.Done():
		log.Printf("[warn] timed out waiting for proxy to close")
		return nil
	case <-proxyClosed:
		return nil
	}
}

// UpdateRules updates proxy rules.
func (p *Server) UpdateRules(rules Rules) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.rules = rules
	log.Printf("[debug] set %d proxy rules", len(rules))
	return nil
}

// Rules returns current proxy rules.
func (p *Server) Rules() Rules {
	p.mu.RLock()
	defer p.mu.RUnlock()

	rules := make(Rules, len(p.rules))
	copy(rules, p.rules)
	return rules
}

func (p *Server) handleRulesRequest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rules" {
		http.NotFound(w, r)
		return
	}

	switch {
	case r.Method == http.MethodOptions:
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusOK)
	case r.Method == http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(p.Rules()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case r.Method == http.MethodPost:
		var rules Rules
		if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var filtered Rules

		for _, r := range rules {
			if r.Host == "" {
				log.Printf("[WARN] empty host in rule %d: %+v, ignoring", len(p.Rules()), r)
				continue
			}

			if len(r.AddHeaders) == 0 {
				log.Printf("[WARN] empty add_headers in rule %d: %+v, ignoring", len(p.Rules()), r)
				continue
			}

			filtered = append(filtered, r)
		}

		if err := p.UpdateRules(filtered); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
