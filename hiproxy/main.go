package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	aglog "github.com/AdguardTeam/golibs/log"
	"github.com/Semior001/headit-safari/hiproxy/proxy"
	"github.com/adrg/xdg"
	"github.com/hashicorp/logutils"
	"github.com/jessevdk/go-flags"
	"gopkg.in/natefinch/lumberjack.v2"
)

var opts struct {
	APIPort   int    `long:"api-port"   env:"API_PORT"   description:"API port to listen on"   default:"9096"`
	ProxyPort int    `long:"proxy-port" env:"PROXY_PORT" description:"Proxy port to listen on" default:"9095"`
	LogFile   string `long:"log-file"   env:"LOG_FILE"   description:"Log file location, default at xdg config home"`
	Insecure  bool   `long:"insecure"   env:"INSECURE"   description:"Don't check TLS connections"`
	Debug     bool   `long:"debug"      env:"DEBUG"      description:"Turn on debug mode"`
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
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { // catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		sig := <-stop
		aglog.Info("received signal %v, shutting down", sig)
		cancel()
	}()

	setupLog(ctx, opts.LogFile, opts.Debug)
	log.Printf("[info] headit-safari, version: %s", getVersion())

	if err := run(ctx); err != nil {
		aglog.Error("failed to %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cert, rules, err := load()
	if err != nil {
		return fmt.Errorf("setup config: %w", err)
	}

	p := proxy.Server{
		APIPort:     opts.APIPort,
		ProxyPort:   opts.ProxyPort,
		Insecure:    opts.Insecure,
		Version:     getVersion(),
		Certificate: cert,
	}
	if err = p.UpdateRules(rules); err != nil {
		return fmt.Errorf("set proxy rules: %w", err)
	}

	if err := p.Run(); err != nil {
		return fmt.Errorf("run proxy: %w", err)
	}

	<-ctx.Done()

	defer func() {
		if err = save(p.Rules()); err != nil {
			log.Printf("[warn] failed to save rules: %v", err)
		}
	}()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err = p.Close(shutdownCtx); err != nil {
		return fmt.Errorf("close proxy: %w", err)
	}

	return nil
}

func save(rules proxy.Rules) error {
	rulesLoc := filepath.Join(xdg.ConfigHome, "headit", "rules.json")

	f, err := os.Create(rulesLoc)
	if err != nil {
		return fmt.Errorf("create rules file: %w", err)
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(rules); err != nil {
		return fmt.Errorf("encode rules: %w", err)
	}

	log.Printf("[info] saved %d rules", len(rules))

	return nil
}

func load() (cert tls.Certificate, rules proxy.Rules, err error) {
	dirLoc := filepath.Join(xdg.ConfigHome, "headit")
	if err := os.MkdirAll(dirLoc, 0755); err != nil && !os.IsExist(err) {
		return cert, rules, fmt.Errorf("create config dir: %w", err)
	}

	if cert, err = proxy.LoadCert(dirLoc); err != nil {
		return cert, rules, fmt.Errorf("load cert: %w", err)
	}

	rulesLoc := filepath.Join(dirLoc, "rules.json")

	f, err := os.Open(rulesLoc)
	if err != nil {
		if os.IsNotExist(err) {
			return cert, rules, nil
		}

		return cert, rules, fmt.Errorf("open rules file: %w", err)
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(&rules); err != nil {
		return cert, rules, fmt.Errorf("decode rules: %w", err)
	}

	return cert, rules, nil
}

func setupLog(ctx context.Context, logFile string, dbg bool) {
	lf := setupLogFile(ctx, logFile)

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"debug", "info", "warn", "error"},
		MinLevel: "info",
		Writer:   os.Stderr,
	}

	logFlags := log.Ldate | log.Ltime

	// this is the level that is set for library's logger,
	// further filtering is done by logutils
	aglog.SetLevel(aglog.DEBUG)

	if dbg {
		logFlags = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile
		filter.MinLevel = "debug"
	}

	log.SetFlags(logFlags)
	log.SetOutput(io.MultiWriter(filter, lf))
}

func setupLogFile(ctx context.Context, loc string) io.Writer {
	if loc == "" {
		dirLoc := filepath.Join(xdg.ConfigHome, "headit")
		if err := os.MkdirAll(dirLoc, 0755); err != nil && !os.IsExist(err) {
			_, _ = fmt.Fprintf(os.Stderr, "failed to create config dir: %v\n", err)
			return io.Discard
		}
		loc = filepath.Join(dirLoc, "headit.log")
	}

	lj := &lumberjack.Logger{
		Filename:   loc,
		LocalTime:  true,
		MaxSize:    10, // MB
		MaxBackups: 1,
	}
	go func() {
		<-ctx.Done()
		_ = lj.Close()
	}()

	return lj
}
