package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
)

var (
	instance *Logger
	once     sync.Once
)

type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errLogger   *log.Logger
	debugLogger *log.Logger
	fatalLogger *log.Logger
	file        *os.File
	config      *config.LogConfig
}

func Init(cfg *config.LogConfig) error {
	var err error
	once.Do(func() {
		instance = &Logger{config: cfg}

		if cfg.Path != "" {
			logDir := filepath.Dir(cfg.Path)
			if logDir != "." && logDir != "" {
				if err = os.MkdirAll(logDir, 0755); err != nil {
					return
				}
			}
			instance.file, err = os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				return
			}
		}

		output := instance.file
		if output == nil {
			output = os.Stdout
		}

		instance.infoLogger = log.New(output, "", 0)
		instance.warnLogger = log.New(output, "", 0)
		instance.errLogger = log.New(output, "", 0)
		instance.debugLogger = log.New(output, "", 0)
		instance.fatalLogger = log.New(output, "", 0)
	})

	return err
}

func Close() {
	if instance != nil && instance.file != nil {
		_ = instance.file.Close()
	}
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func Infof(format string, v ...interface{}) {
	if instance != nil {
		instance.infoLogger.Printf("%s [INFO] %s", timestamp(), fmt.Sprintf(format, v...))
	}
}

func Warnf(format string, v ...interface{}) {
	if instance != nil {
		instance.warnLogger.Printf("%s [WARN] %s", timestamp(), fmt.Sprintf(format, v...))
	}
}

func Errorf(format string, v ...interface{}) {
	if instance != nil {
		instance.errLogger.Printf("%s [ERROR] %s", timestamp(), fmt.Sprintf(format, v...))
	}
}

func Debugf(format string, v ...interface{}) {
	if instance != nil && (instance.config.Level == "debug" || instance.config.Level == "info") {
		instance.debugLogger.Printf("%s [DEBUG] %s", timestamp(), fmt.Sprintf(format, v...))
	}
}

func InfofWithTraceID(traceID, format string, v ...interface{}) {
	if instance != nil {
		instance.infoLogger.Printf("%s [INFO] [TraceID: %s] %s", timestamp(), traceID, fmt.Sprintf(format, v...))
	}
}

func ErrorfWithTraceID(traceID, format string, v ...interface{}) {
	if instance != nil {
		instance.errLogger.Printf("%s [ERROR] [TraceID: %s] %s", timestamp(), traceID, fmt.Sprintf(format, v...))
	}
}

func Fatalf(format string, v ...interface{}) {
	if instance != nil {
		instance.fatalLogger.Printf("%s [FATAL] %s", timestamp(), fmt.Sprintf(format, v...))
	}
	os.Exit(1)
}
