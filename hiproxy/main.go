package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/hashicorp/logutils"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	ProxyAddr    string `long:"proxy-addr" description:"Proxy address to listen on" default:"localhost:9095"`
	ProviderAddr string `long:"provider-addr" description:"Provider address to listen on" default:"localhost:9096"`
	SSL          struct {
		GenerateCert string `long:"generate-cert" description:"Generate self-signed certificate and exit, value is the location to save cert and key files"`
		CertLocation string `long:"cert-loc" description:"Location of the certificate file" default:"./cert.pem"`
		KeyLocation  string `long:"key-loc" description:"Location of the key file" default:"./key.pem"`
	} `group:"SSL Options" namespace:"ssl" env-namespace:"SSL" env-group:"SSL"`
	Debug bool `long:"debug" description:"Turn on debug mode"`
}

var version = "unknown"

func getVersion() string {
	if version != "unknown" {
		return version
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	return bi.Main.Version
}

func main() {
	_, _ = fmt.Fprintf(os.Stderr, "headit-safari-proxy, version %s\n", getVersion())

	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	setupLog(opts.Debug)

	if err := run(context.Background()); err != nil {
		log.Printf("[ERROR] failed to %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if opts.SSL.GenerateCert != "" {
		if err := generateCert(opts.SSL.GenerateCert); err != nil {
			return fmt.Errorf("generate cert: %w", err)
		}
		return nil
	}

	p := Provider{Addr: opts.ProviderAddr, Version: getVersion()}

	rulesCh, err := p.Run(ctx)
	if err != nil {
		return fmt.Errorf("start provider: %w", err)
	}

	pr := Proxy{Addr: opts.ProxyAddr, Version: getVersion()}
	pr.SSL.CertLocation = opts.SSL.CertLocation
	pr.SSL.KeyLocation = opts.SSL.KeyLocation
	if err := pr.Run(ctx, rulesCh); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}

	return nil
}

func setupLog(dbg bool) {
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: "INFO",
		Writer:   os.Stderr,
	}

	logFlags := log.Ldate | log.Ltime

	if dbg {
		logFlags = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
		filter.MinLevel = "DEBUG"
	}

	log.SetFlags(logFlags)
	log.SetOutput(filter)
}
