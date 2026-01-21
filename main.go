package main

import (
	"log"
	"net/http"
	"os" // ← КРИТИЧЕСКИ ВАЖНЫЙ ИМПОРТ
)

var logger *AppLogger

func main() {
	logger = NewLogger()
	logger.InfoLogger.Println("🚀 Сервер запускается...")

	// ПРИНУДИТЕЛЬНО СБРАСЫВАЕМ БУФЕР
	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	http.HandleFunc("/goals", func(w http.ResponseWriter, r *http.Request) {
		// ... существующий код
	})

	http.HandleFunc("/goals/", func(w http.ResponseWriter, r *http.Request) {
		// ... существующий код
	})

	port := "8080"
	logger.InfoLogger.Printf("📡 Сервер запущен на http://localhost:%s/goals", port)

	// ЕЩЁ РАЗ СБРАСЫВАЕМ БУФЕР
	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
