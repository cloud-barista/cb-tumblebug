package logger

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	sharedLogFile *lumberjack.Logger
)

func init() {

	// Map environment variable names to config file key names
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)

	viper.AutomaticEnv()

	// Set config values
	logLevel := viper.GetString("loglevel")
	env := viper.GetString("node.env")

	// Set the global logger to use JSON format.
	zerolog.TimeFieldFormat = time.RFC3339

	// Get log file configuration from environment variables
	logFilePath, maxSize, maxBackups, maxAge, compress := getLogFileConfig()

	// Initialize a shared log file with lumberjack to manage log rotation
	sharedLogFile = &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   compress,
	}

	// Set the log level
	var level zerolog.Level
	switch logLevel {
	case "trace":
		level = zerolog.TraceLevel
	case "debug":
		level = zerolog.DebugLevel
	case "info":
		level = zerolog.InfoLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	case "fatal":
		level = zerolog.FatalLevel
	case "panic":
		level = zerolog.PanicLevel
	default:
		log.Warn().Msgf("Invalid LOGLEVEL value: %s. Using default value: info", logLevel)
		level = zerolog.InfoLevel
	}

	logger := NewLogger(level)

	// Set global logger
	log.Logger = *logger

	// Check the execution environment from the environment variable
	// Log a message
	log.Info().
		Str("logLevel", level.String()).
		Str("env", env).
		Int("maxSize", maxSize).
		Int("maxBackups", maxBackups).
		Int("maxAge", maxAge).
		Bool("compress", compress).
		Msg("Global logger initialized")
}

// Create a new logger
func NewLogger(level zerolog.Level) *zerolog.Logger {

	// Set config values
	logwriter := viper.GetString("logwriter")

	// Multi-writer setup: logs to both file and console
	multi := zerolog.MultiLevelWriter(
		sharedLogFile,
		zerolog.ConsoleWriter{Out: os.Stdout},
	)

	var logger zerolog.Logger

	// Check the execution environment from the environment variable
	// Configure the log output
	if logwriter == "both" {
		// Apply file to the global logger
		logger = zerolog.New(multi).Level(level).With().Timestamp().Caller().Logger()
	} else if logwriter == "file" {
		// Apply file writer to the global logger
		logger = zerolog.New(sharedLogFile).Level(level).With().Timestamp().Caller().Logger()
	} else if logwriter == "stdout" {
		// Apply ConsoleWriter to the global logger
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).Level(level).With().Timestamp().Caller().Logger()
	} else {
		log.Warn().Msgf("Invalid LOGWRITER value: %s. Using default value: both", logwriter)
		// Apply multi-writer to the global logger
		logger = zerolog.New(multi).Level(level).With().Timestamp().Caller().Logger()
	}

	// Log a message
	logger.Info().
		Str("logLevel", level.String()).
		Msg("New logger created")

	if logwriter == "file" {
		logger.Info().
			Str("logFilePath", sharedLogFile.Filename).
			Msg("Single-write setup (logs to file only)")

	} else if logwriter == "stdout" {
		logger.Info().
			Str("ConsoleWriter", "os.Stdout").
			Msg("Single-write setup (logs to console only)")
	} else {
		logger.Info().
			Str("logFilePath", sharedLogFile.Filename).
			Str("ConsoleWriter", "os.Stdout").
			Msg("Multi-writes setup (logs to both file and console)")
	}

	return &logger
}

// Get log file configuration from environment variables
func getLogFileConfig() (string, int, int, int, bool) {

	// Set config values
	logFilePath := viper.GetString("logfile.path")

	// Default: ./log/tumblebug.log
	if logFilePath == "" {
		log.Warn().Msg("LOGFILE_PATH is not set. Using default value: ./log/tumblebug.log")
		logFilePath = "./log/tumblebug.log"
	}

	// Default: 10 MB
	maxSize, err := strconv.Atoi(viper.GetString("logfile.maxsize"))
	if err != nil {
		log.Warn().Msgf("Invalid LOGFILE_MAXSIZE value: %s. Using default value: 10 MB", viper.GetString("logfile.maxsize"))
		maxSize = 10
	}

	// Default: 3 backups
	maxBackups, err := strconv.Atoi(viper.GetString("logfile.maxbackups"))
	if err != nil {
		log.Warn().Msgf("Invalid LOGFILE_MAXBACKUPS value: %s. Using default value: 3 backups", viper.GetString("logfile.maxbackups"))
		maxBackups = 3
	}

	// Default: 30 days
	maxAge, err := strconv.Atoi(viper.GetString("logfile.maxage"))
	if err != nil {
		log.Warn().Msgf("Invalid LOGFILE_MAXAGE value: %s. Using default value: 30 days", viper.GetString("logfile.maxage"))
		maxAge = 30
	}

	// Default: false
	compress, err := strconv.ParseBool(viper.GetString("logfile.compress"))
	if err != nil {
		log.Warn().Msgf("Invalid LOGFILE_COMPRESS value: %s. Using default value: false", viper.GetString("logfile.compress"))
		compress = false
	}

	return logFilePath, maxSize, maxBackups, maxAge, compress
}
