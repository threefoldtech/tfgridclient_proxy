package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/waleedhammam/graphql-server/pkg/explorer"
)

type flags struct {
	explorer string
	debug    string
	redis    string
}

func main() {
	f := flags{}
	flag.StringVar(&f.explorer, "explorer", "https://explorer.devnet.grid.tf/graphql/", "explorer url")
	flag.StringVar(&f.debug, "log-level", "debug", "log level [debug|info|warn|error|fatal|panic]")
	flag.StringVar(&f.redis, "redis", "127.0.0.1:6379", "redis url")
	flag.Parse()
	setupLogging(f.debug)
	s, err := createServer(f)
	if err != nil {
		panic(err)
	}

	if err := s.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Info().Msg("server stopped gracefully")
		} else {
			log.Error().Err(err).Msg("server stopped unexpectedly")
		}
	}
}

func createServer(f flags) (*http.Server, error) {
	log.Info().Msg("Creating server")
	router := mux.NewRouter()
	debug := false
	explorer_flags := f.explorer
	redis := f.redis
	if f.debug == "all" {
		debug = true
	}

	// setup explorer
	explorer.Setup(router, debug, explorer_flags, redis)

	return &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:8080",
	}, nil
}

const (
	colorBlack = iota + 30
	colorRed
	colorGreen
	colorYellow
	colorBlue
	colorMagenta
	colorCyan
	colorWhite

	colorBold     = 1
	colorDarkGray = 90
)

// colorize returns the string s wrapped in ANSI code c, unless disabled is true.
func colorize(s interface{}, c int) string {
	return fmt.Sprintf("\x1b[%dm%v\x1b[0m", c, s)
}

func formatLevel(i interface{}) string {
	var l string
	if ll, ok := i.(string); ok {
		switch ll {
		case "debug":
			l = colorize(ll, colorBlue)
		case "info":
			l = colorize(ll, colorGreen)
		case "warn":
			l = colorize(ll, colorYellow)
		case "error":
			l = colorize(colorize(ll, colorRed), colorBold)
		case "fatal":
			l = colorize(colorize(ll, colorRed), colorBold)
		case "panic":
			l = colorize(colorize(ll, colorRed), colorBold)
		default:
			l = colorize("???", colorBold)
		}
	} else {
		if i == nil {
			l = colorize("???", colorBold)
		} else {
			l = strings.ToUpper(fmt.Sprintf("%s", i))[0:3]
		}
	}
	return l
}

func setupLogging(level string) {
	if level == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if level == "info" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else if level == "warn" {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	} else if level == "error" {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	} else if level == "fatal" {
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	} else if level == "panic" {
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{
		TimeFormat:  time.RFC3339,
		Out:         os.Stdout,
		FormatLevel: formatLevel,
	})
}