// Package provider contains structures with proxy rules and methods for providers to implement.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/go-pkgz/rest"
	"github.com/go-pkgz/rest/logger"
)

// Provider specifies methods for provider to implement.
type Provider struct {
	Addr    string
	Version string

	srv *http.Server
	ch  chan<- Rules
}

// Run starts provider listener.
// Non-blocking, returns channel to send rules to.
func (p *Provider) Run(ctx context.Context) (<-chan Rules, error) {
	ch := make(chan Rules)
	p.ch = ch

	p.srv = &http.Server{
		Addr: p.Addr,
		Handler: rest.Wrap(http.HandlerFunc(p.handle),
			rest.Ping,
			rest.AppInfo("headit-safari-proxy", "Semior001", p.Version),
			logger.New(
				logger.Prefix("[DEBUG][provider]"),
			).Handler,
		),
		ReadHeaderTimeout: 30 * time.Second,
	}

	go func() {
		<-ctx.Done()

		log.Printf("[DEBUG] shutting down provider")
		if err := p.srv.Shutdown(context.Background()); err != nil {
			log.Printf("[WARN] provider: failed to shutdown server: %v", err)
		}
		close(p.ch)
		p.srv, p.ch = nil, nil
	}()

	go func() {
		log.Printf("[INFO] started provider on %s", p.Addr)
		if err := p.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] provider: failed to start server: %v", err)
		}
	}()

	return ch, nil
}

func (p *Provider) handle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Rules Rules `json:"rules"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("decode response: %v", err), http.StatusBadRequest)
		return
	}

	for i := range req.Rules {
		if req.Rules[i].BaseURL == "" {
			http.Error(w, fmt.Sprintf("[%d]: base_url is required", i), http.StatusBadRequest)
			return
		}
		if _, err := regexp.Compile(req.Rules[i].BaseURL); err != nil {
			http.Error(w, fmt.Sprintf("compile %d's base_url: %v", i, err), http.StatusBadRequest)
			return
		}
		req.Rules[i].CompiledBaseURL = regexp.MustCompile(req.Rules[i].BaseURL)
	}

	// sort rules by base URL length to match the longest first
	sort.Slice(req.Rules, func(i, j int) bool { return len(req.Rules[i].BaseURL) > len(req.Rules[j].BaseURL) })

	p.ch <- req.Rules
	w.WriteHeader(http.StatusOK)
}

// Rules is a set of rules for the specified base URL regex.
type Rules []Rule

// Rule specifies what to do with the request.
type Rule struct {
	// BaseURL is a regex to match the request URL.
	BaseURL string `json:"base_url"`

	// CompiledBaseURL is a compiled regex to match the request URL.
	CompiledBaseURL *regexp.Regexp `json:"-"`

	// AddHeaders specifies headers to add to the request.
	AddHeaders map[string]string `json:"add_headers"`
}
