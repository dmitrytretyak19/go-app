// ФАЙЛ: main.go
// НАЗНАЧЕНИЕ: Точка входа приложения, маршрутизация запросов, инициализация сервисов
// ОСОБЕННОСТИ:
//   - Поддержка Heroku (динамический порт, переменные окружения)
//   - Интеграция с системой безопасности (rate limiting, DDoS protection)
//   - Профессиональное логирование запросов

package main

// ИМПОРТЫ: Все необходимые пакеты (исправлено)
import (
	"context" // ← ДОБАВЛЕН ДЛЯ КОНТЕКСТА
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// PostgreSQL драйвер
	"github.com/jackc/pgx/v5"
)

// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ
var (
	logger *AppLogger // Основной логгер приложения
	dbURL  string     // Строка подключения к базе данных
)

// ОСНОВНАЯ ФУНКЦИЯ ПРИЛОЖЕНИЯ
func main() {
	// ШАГ 1: ИНИЦИАЛИЗИРУЕМ ЛОГГЕР
	logger = NewLogger()
	logger.InfoLogger.Println("🚀 Сервер запускается...")

	// Принудительно сбрасываем буфер для немедленного отображения
	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// ШАГ 2: ИНИЦИАЛИЗИРУЕМ СИСТЕМУ БЕЗОПАСНОСТИ
	initSecurity()
	logger.InfoLogger.Println("🛡️ Система безопасности активирована")
	initMetrics()
	registerMetricsEndpoint()

	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// ШАГ 3: НАСТРАИВАЕМ ПОДКЛЮЧЕНИЕ К БАЗЕ ДАННЫХ
	setupDatabase()
	logger.InfoLogger.Println("🗄️ Подключение к базе данных настроено")

	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// ШАГ 4: РЕГИСТРИРУЕМ ОБРАБОТЧИКИ С MIDDLEWARE БЕЗОПАСНОСТИ
	registerHandlers()
	logger.InfoLogger.Println("🔌 Обработчики запросов зарегистрированы")

	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// ШАГ 5: ОПРЕДЕЛЯЕМ ПОРТ ДЛЯ ЗАПУСКА
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Порт по умолчанию для локальной разработки
		logger.InfoLogger.Printf("⚠️ PORT не задан в переменных окружения, используем порт %s", port)
	} else {
		logger.InfoLogger.Printf("ℹ️ Используем порт из переменных окружения: %s", port)
	}

	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// ШАГ 6: ЗАПУСКАЕМ СЕРВЕР
	address := ":" + port
	logger.InfoLogger.Printf("📡 Сервер запущен на http://0.0.0.0:%s/goals", port)

	// Принудительная синхронизация перед запуском сервера
	if file, ok := logger.InfoLogger.Writer().(*os.File); ok {
		file.Sync()
	}

	// КРИТИЧЕСКИ ВАЖНО: Слушаем все интерфейсы (0.0.0.0), а не только localhost
	err := http.ListenAndServe(address, nil)
	if err != nil {
		logger.LogError(err, "КРИТИЧЕСКАЯ ОШИБКА: Сервер не запущен")
		log.Fatalf("❌ Сервер завершил работу с ошибкой: %v", err)
	}
}

// ФУНКЦИЯ: setupDatabase
// НАЗНАЧЕНИЕ: Настраивает подключение к базе данных
func setupDatabase() {
	// Получаем строку подключения из переменных окружения (Heroku)
	dbURL = os.Getenv("DATABASE_URL")

	// Для локальной разработки используем тестовую базу
	if dbURL == "" {
		logger.InfoLogger.Println("ℹ️ DATABASE_URL не задан, используем локальную базу данных")
		dbURL = "postgres://myuser@localhost:5432/mydb?sslmode=disable"
	} else {
		// Для Heroku добавляем sslmode=require
		if !strings.Contains(dbURL, "sslmode=") {
			if strings.Contains(dbURL, "?") {
				dbURL += "&sslmode=require"
			} else {
				dbURL += "?sslmode=require"
			}
			logger.InfoLogger.Println("ℹ️ Добавлен параметр sslmode=require для Heroku")
		}
	}

	// Проверка подключения к базе данных
	logger.InfoLogger.Printf("🔍 Проверяем подключение к базе данных: %s", maskDBURL(dbURL))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.LogError(err, "ОШИБКА ПОДКЛЮЧЕНИЯ К БАЗЕ ДАННЫХ")
		log.Fatalf("❌ Не удалось подключиться к базе данных: %v", err)
	}
	defer conn.Close(ctx)

	logger.InfoLogger.Println("✅ Подключение к базе данных успешно установлено")
}

// ФУНКЦИЯ: registerHandlers
// НАЗНАЧЕНИЕ: Регистрирует все обработчики с middleware безопасности
func registerHandlers() {
	// Обработчик для /goals
	http.Handle("/goals", securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.LogRequest(r.Method, r.URL.Path, 0)

		// Логируем IP-адрес для безопасности
		ip := getIP(r)
		logger.InfoLogger.Printf("🌐 Запрос от IP: %s | User-Agent: %s",
			ip, r.Header.Get("User-Agent"))

		switch r.Method {
		case http.MethodGet:
			getGoalsHandler(w, r)
		case http.MethodPost:
			createGoalHandler(w, r)
		default:
			logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
			http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		}
	})))

	// Обработчик для /goals/
	http.Handle("/goals/", securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.LogRequest(r.Method, r.URL.Path, 0)

		// Логируем IP-адрес для безопасности
		ip := getIP(r)
		logger.InfoLogger.Printf("🌐 Запрос от IP: %s | User-Agent: %s",
			ip, r.Header.Get("User-Agent"))

		switch r.Method {
		case http.MethodPut:
			updateGoalHandler(w, r)
		case http.MethodDelete:
			deleteGoalHandler(w, r)
		default:
			logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
			http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		}
	})))

	// Обработчик для корневого пути (для удобства)
	http.Handle("/", securityMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			logger.LogRequest(r.Method, r.URL.Path, http.StatusNotFound)
			http.NotFound(w, r)
			return
		}

		logger.LogRequest(r.Method, r.URL.Path, http.StatusOK)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
			<title>API для управления целями</title>
			<style>
				body { font-family: Arial, sans-serif; margin: 40px; line-height: 1.6; }
				h1 { color: #2c3e50; }
				.endpoint { background: #f8f9fa; padding: 15px; margin: 10px 0; border-radius: 5px; }
				.method { display: inline-block; width: 80px; text-align: center; padding: 3px; 
				          border-radius: 3px; color: white; font-weight: bold; }
				.get { background: #28a745; } .post { background: #007bff; } 
				.put { background: #ffc107; color: #212529; } .delete { background: #dc3545; }
				.footer { margin-top: 30px; color: #6c757d; font-size: 14px; }
			</style>
		</head>
		<body>
			<h1>🎯 API для управления целями</h1>
			<p>Документация по endpoint'ам:</p>
			
			<div class="endpoint">
				<span class="method get">GET</span> <strong>/goals</strong> - Получение всех целей
			</div>
			<div class="endpoint">
				<span class="method post">POST</span> <strong>/goals</strong> - Создание новой цели
			</div>
			<div class="endpoint">
				<span class="method put">PUT</span> <strong>/goals/{id}</strong> - Обновление цели
			</div>
			<div class="endpoint">
				<span class="method delete">DELETE</span> <strong>/goals/{id}</strong> - Удаление цели
			</div>
			
			<div class="footer">
				<p>Сервер запущен: <strong>` + time.Now().Format(time.RFC3339) + `</strong></p>
				<p>Защита от DDoS-атак активна ✅</p>
			</div>
		</body>
		</html>
		`))
	})))
}

// ФУНКЦИЯ: maskDBURL
// НАЗНАЧЕНИЕ: Маскирует пароль в строке подключения для логов
func maskDBURL(url string) string {
	if strings.Contains(url, "@") {
		parts := strings.Split(url, "@")
		if len(parts) > 1 {
			hostPart := strings.Split(parts[1], "/")[0]
			return "postgres://*******@" + hostPart + "/..."
		}
	}
	return url
}

// ФУНКЦИЯ: getIP
// НАЗНАЧЕНИЕ: Получает реальный IP-адрес клиента (учитывая прокси)
