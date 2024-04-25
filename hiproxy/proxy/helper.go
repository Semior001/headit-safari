package proxy

import "crypto/tls"

type loggerFn func(string, ...interface{})

func (l loggerFn) Printf(format string, v ...interface{}) { l(format, v...) }

type staticCert tls.Certificate

func (s staticCert) Fetch(string, func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	return (*tls.Certificate)(&s), nil
}
