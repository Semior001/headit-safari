module github.com/Semior001/headit-safari/hiproxy

go 1.22.2

replace github.com/AdguardTeam/gomitmproxy => github.com/Semior001/gomitmproxy v0.0.0-20240819114105-06c714dfa274

require (
	github.com/AdguardTeam/golibs v0.23.2
	github.com/AdguardTeam/gomitmproxy v0.2.1
	github.com/adrg/xdg v0.4.0
	github.com/go-chi/cors v1.2.1
	github.com/go-pkgz/rest v1.19.0
	github.com/hashicorp/logutils v1.0.0
	github.com/jessevdk/go-flags v1.5.0
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require (
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
