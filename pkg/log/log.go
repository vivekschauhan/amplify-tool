package log

import (
	"time"

	"github.com/sirupsen/logrus"
)

const (
	lineFormat = "line"
	jsonFormat = "json"
)

func getFormatter(format string) logrus.Formatter {
	if format == lineFormat {
		return &logrus.TextFormatter{
			TimestampFormat:  time.RFC3339,
			FullTimestamp:    true,
			PadLevelText:     true,
			QuoteEmptyFields: true,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyMsg: "message",
			},
		}
	}
	return &logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyMsg: "message",
		},
	}
}

// setupLogging sets up logging for each used package
func GetLogger(level string, format string) *logrus.Logger {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}

	formatter := getFormatter(format)
	logger := logrus.New()
	logger.Level = lvl
	logger.Formatter = formatter
	return logger
}
