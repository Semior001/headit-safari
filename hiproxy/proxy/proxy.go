// Package proxy contains services and types for the proxy server.
package proxy

import "regexp"

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
