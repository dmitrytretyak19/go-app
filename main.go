package main

import (
	"log"
	"net/http"
	"os"
)

var logger *AppLogger
var dbURL string

func main() {
	// ШАГ 1: ИНИЦИАЛИЗИРУЕМ ЛОГГЕР
	logger = NewLogger()
	logger.InfoLogger.Println("🚀 Сервер запускается...")

	// ШАГ 2: ОПРЕДЕЛЯЕМ СТРОКУ ПОДКЛЮЧЕНИЯ К БД
	// Используем DATABASE_URL из Heroku, если она есть
	dbURL = os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Для локальной разработки
		dbURL = "postgres://myuser@localhost:5432/mydb?sslmode=disable"
	}

	// ШАГ 3: РЕГИСТРИРУЕМ ОБРАБОТЧИКИ
	http.HandleFunc("/goals", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getGoalsHandler(w, r)
		case http.MethodPost:
			createGoalHandler(w, r)
		default:
			logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
			http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/goals/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			updateGoalHandler(w, r)
		case http.MethodDelete:
			deleteGoalHandler(w, r)
		default:
			logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
			http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		}
	})

	// ШАГ 4: ОПРЕДЕЛЯЕМ ПОРТ
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Для локальной разработки
	}

	// ШАГ 5: ЗАПУСКАЕМ СЕРВЕР
	// ВАЖНО: Слушаем ВСЕ ИНТЕРФЕЙСЫ (0.0.0.0), а не localhost!
	address := ":" + port
	logger.InfoLogger.Printf("📡 Сервер запущен на http://0.0.0.0%s/goals", port)

	log.Fatal(http.ListenAndServe(address, nil))
}
