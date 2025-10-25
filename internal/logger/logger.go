package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var (
	currentLevel = INFO
	logFile      *os.File
	levelNames   = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
	}
	levelColors = map[LogLevel]string{
		DEBUG: "\033[36m",
		INFO:  "\033[32m",
		WARN:  "\033[33m",
		ERROR: "\033[31m",
	}
	resetColor = "\033[0m"
)

func Init() {
	log.SetFlags(log.Ldate | log.Ltime)

	logFilePath := os.Getenv("LOG_FILE")
	if logFilePath != "" {
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("[ERROR] Failed to create log directory: %v", err)
		} else {
			file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				log.Printf("[ERROR] Failed to open log file: %v", err)
			} else {
				logFile = file
				multiWriter := io.MultiWriter(os.Stdout, logFile)
				log.SetOutput(multiWriter)
				log.Printf("[INFO] Log file enabled: %s", logFilePath)
			}
		}
	}

	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "INFO"
	}

	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		currentLevel = DEBUG
	case "INFO":
		currentLevel = INFO
	case "WARN":
		currentLevel = WARN
	case "ERROR":
		currentLevel = ERROR
	default:
		currentLevel = INFO
		log.Printf("[WARN] Invalid LOG_LEVEL '%s', using INFO", levelStr)
	}

	log.Printf("[%s] Log level set to %s", levelNames[INFO], levelNames[currentLevel])
}

func logf(level LogLevel, format string, args ...interface{}) {
	if level >= currentLevel {
		msg := fmt.Sprintf(format, args...)
		log.Printf("%s[%s]%s %s", levelColors[level], levelNames[level], resetColor, msg)
	}
}

func Debug(format string, args ...interface{}) {
	logf(DEBUG, format, args...)
}

func Info(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

func Warn(format string, args ...interface{}) {
	logf(WARN, format, args...)
}

func Error(format string, args ...interface{}) {
	logf(ERROR, format, args...)
}

func Println(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

func Printf(format string, args ...interface{}) {
	logf(INFO, format, args...)
}

func IsDebug() bool {
	return currentLevel <= DEBUG
}

func Close() {
	if logFile != nil {
		logFile.Close()
	}
}
