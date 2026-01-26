// ФАЙЛ: alerts.go
// НАЗНАЧЕНИЕ: Система алертинга и уведомлений
// ОСОБЕННОСТИ:
//   - Отправка уведомлений в Telegram
//   - Автоматическая блокировка подозрительных IP
//   - Нормализация IP-адресов для корректного подсчёта ошибок

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ДЛЯ АЛЕРТИНГА
var (
	// Хранилище ошибок
	errorCounts = make(map[string]int)
	// Мьютекс для потокобезопасности
	alertMutex sync.Mutex
	// Telegram бот токен (из переменных окружения)
	telegramBotToken string
	// Telegram чат ID (из переменных окружения)
	telegramChatID string
	// Порог ошибок для отправки алерта
	errorThreshold = 5
)

// ИНИЦИАЛИЗАЦИЯ АЛЕРТИНГА
func initAlerts() {
	// Получаем данные из переменных окружения
	telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID = os.Getenv("TELEGRAM_CHAT_ID")

	if telegramBotToken == "" || telegramChatID == "" {
		logger.InfoLogger.Println("⚠️ TELEGRAM_BOT_TOKEN или TELEGRAM_CHAT_ID не заданы, алертинг отключен")
		return
	}

	logger.InfoLogger.Println("🔔 Система алертинга активирована")

	// Запускаем фоновый мониторинг
	go monitorErrors()
}

// ФУНКЦИЯ: Логирование ошибок с алертингом
func logErrorWithAlert(errorMsg string, context string, ip string) {
	logger.InfoLogger.Printf("DEBUG: logErrorWithAlert called with IP: %s", ip)

	// Нормализуем IP для корректного подсчёта
	normalizedIP := normalizeIP(ip)

	// Логируем ошибку
	logger.InfoLogger.Printf("ALERT: %s | Error: %s | IP: %s", context, errorMsg, normalizedIP)

	// Если Telegram не настроен — выходим
	if telegramBotToken == "" || telegramChatID == "" {
		return
	}

	// Увеличиваем счётчик ошибок для этого IP
	alertMutex.Lock()
	errorCounts[normalizedIP]++
	currentCount := errorCounts[normalizedIP]
	// Добавляем DEBUG лог для отладки
	logger.InfoLogger.Printf("DEBUG: Error count for IP %s = %d", normalizedIP, currentCount)
	alertMutex.Unlock()

	// Если превышен порог — отправляем алерт
	if currentCount >= errorThreshold {
		sendTelegramAlert(context, normalizedIP, currentCount)
		blockSuspiciousIP(normalizedIP)
	}
}

// ФУНКЦИЯ: Отправка алерта в Telegram
func sendTelegramAlert(context, ip string, count int) {
	// Формируем сообщение
	message := "🚨 ALERT: High error rate detected!\n" +
		"Context: " + context + "\n" +
		"IP: " + ip + "\n" +
		"Error count: " + fmt.Sprintf("%d", count) + "\n" +
		"Time: " + time.Now().Format(time.RFC3339)

	// Формируем URL для Telegram API
	url := "https://api.telegram.org/bot" + telegramBotToken + "/sendMessage"

	// Подготавливаем данные
	payload := map[string]string{
		"chat_id": telegramChatID,
		"text":    message,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.LogError(err, "Ошибка формирования JSON для Telegram алерта")
		return
	}

	// Отправляем запрос
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logger.LogError(err, "Ошибка отправки Telegram алерта")
		return
	}
	defer resp.Body.Close()

	logger.InfoLogger.Printf("✅ Telegram алерт отправлен для IP: %s", ip)
}

// ФУНКЦИЯ: Блокировка подозрительного IP
func blockSuspiciousIP(ip string) {
	// Добавляем IP в список заблокированных
	countMutex.Lock()
	blockedIPs[ip] = time.Now()
	countMutex.Unlock()

	logger.InfoLogger.Printf("🔒 IP %s заблокирован за подозрительную активность", ip)

	// Логируем в security.log
	logSecurityEvent("SUSPICIOUS_IP_BLOCKED", ip, "high_error_rate")
}

// ФУНКЦИЯ: Мониторинг ошибок в фоне
func monitorErrors() {
	for {
		time.Sleep(1 * time.Minute)

		// Очищаем старые записи
		alertMutex.Lock()
		for ip, count := range errorCounts {
			if count < errorThreshold {
				delete(errorCounts, ip)
			}
		}
		alertMutex.Unlock()
	}
}

// ФУНКЦИЯ: Обновление middleware для обработки ошибок
func alertMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				ip := getIP(r)
				// Преобразуем любое значение в строку
				var errorMsg string
				switch e := err.(type) {
				case string:
					errorMsg = e
				case error:
					errorMsg = e.Error()
				default:
					errorMsg = "Unknown panic"
				}
				logErrorWithAlert(errorMsg, "PANIC in request handler", ip)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// ФУНКЦИЯ: Нормализация IP-адресов
func normalizeIP(ip string) string {
	// Преобразуем IPv6 localhost в IPv4
	if ip == "::1" || ip == "[::1]" {
		return "127.0.0.1"
	}

	// Убираем порт из IPv6 адресов
	if strings.HasPrefix(ip, "[") && strings.Contains(ip, "]") {
		end := strings.Index(ip, "]")
		if end != -1 {
			ip = ip[1:end]
		}
	}

	// Убираем порт из IPv4 адресов
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) > 1 {
			ip = parts[0]
		}
	}

	return ip
}
