package platform

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

// NewLogger 创建一个带级别和环境标签的 zerolog 实例。
func NewLogger(level, env string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// dev 用 console writer 方便阅读；prod 用 JSON
	if env == "dev" {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
			Timestamp().Str("env", env).Logger()
	}
	return zerolog.New(os.Stderr).With().Timestamp().Str("env", env).Logger()
}
