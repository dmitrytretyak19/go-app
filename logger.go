package main

import (
	"io"
	"log"
	"os"
	"runtime/debug"
)

type AppLogger struct {
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
}

func NewLogger() *AppLogger {
	// Создаем файл логов
	logFile, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("❌ Ошибка создания файла логов: %v", err)
	}

	// Создаем MultiWriter: пишем И в файл, И в консоль
	multiWriter := io.MultiWriter(logFile, os.Stdout)

	// Настраиваем логгеры
	infoLogger := log.New(multiWriter, "INFO: ", log.Ldate|log.Ltime|log.LUTC)
	errorLogger := log.New(multiWriter, "ERROR: ", log.Ldate|log.Ltime|log.LUTC|log.Lshortfile)

	return &AppLogger{
		InfoLogger:  infoLogger,
		ErrorLogger: errorLogger,
	}
}

// МЕТОД ДЛЯ ЛОГИРОВАНИЯ ЗАПРОСОВ
func (l *AppLogger) LogRequest(method, path string, status int) {
	l.InfoLogger.Printf("%s %s %d", method, path, status)
}

// МЕТОД ДЛЯ ЛОГИРОВАНИЯ ОШИБОК
func (l *AppLogger) LogError(err error, context string) {
	if err != nil {
		l.ErrorLogger.Printf("%s: %v\n%s", context, err, debug.Stack())
	} else {
		l.ErrorLogger.Printf("%s", context)
	}
}
