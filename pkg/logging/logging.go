package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

var logger *logrus.Logger

// SetupLogging initializes the logger with the appropriate settings.
func SetupLogging() *logrus.Logger {
	// Get the debug from environment variable
	debug := false
	if os.Getenv("DEBUG") == "true" {
		debug = true
	}
	if logger == nil {
		logger = logrus.New()
		logger.SetOutput(os.Stdout)
		logger.SetReportCaller(true)

		// Initialize a custom log formatter without timestamps
		customFormatter := new(logrus.TextFormatter)
		customFormatter.DisableTimestamp = true // Disable timestamp since k8s will handle it
		customFormatter.FullTimestamp = false
		logger.SetFormatter(customFormatter)

		// Set the logging level based on the debug environment variable
		if debug {
			logger.SetLevel(logrus.DebugLevel)
		} else {
			logger.SetLevel(logrus.InfoLevel)
		}
	}

	return logger
}
