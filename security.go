// ФАЙЛ: security.go
// НАЗНАЧЕНИЕ: Защита от атак и ограничение запросов
// ОСОБЕННОСТИ:
//   - Автоматическое блокирование IP
//   - Гибкие лимиты для разных endpoint'ов
//   - Интеграция с логированием

package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ДЛЯ ЗАЩИТЫ
var (
	// Хранилище запросов: IP → количество запросов
	requestCounts = make(map[string]int)
	// Хранилище времени последнего запроса
	lastRequestTime = make(map[string]time.Time)
	// Мапа заблокированных IP
	blockedIPs = make(map[string]time.Time)
	// Мьютекс для потокобезопасности
	countMutex sync.Mutex
	// Белый список IP (разрешены без лимитов)
	trustedIPs = []string{
		"127.0.0.1", // Локальный хост
		"::1",       // IPv6 локальный хост
		"10.0.0.1",  // Внутренний IP офиса
	}
	// Лимиты запросов
	requestLimit   = 100           // Максимум запросов в минуту
	blockDuration  = 1 * time.Hour // Время блокировки
	securityLogger *log.Logger     // Отдельный логгер для безопасности
)

// ИНИЦИАЛИЗАЦИЯ ЗАЩИТЫ
func initSecurity() {
	// Создаём отдельный лог-файл для безопасности
	securityFile, err := os.OpenFile("security.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("❌ Не удалось создать security.log: %v", err)
	}
	securityLogger = log.New(securityFile, "SECURITY: ", log.Ldate|log.Ltime|log.LUTC)

	// Запускаем очистку старых записей каждые 5 минут
	go cleanRequestCounts()
}

// MIDDLEWARE: Rate limiting и защита от DDoS
func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)

		// ШАГ 1: Проверяем белый список
		if isTrusted(ip) {
			next.ServeHTTP(w, r)
			return
		}

		// ШАГ 2: Проверяем блокировку
		if isBlocked(ip) {
			logSecurityEvent("BLOCKED_ACCESS", ip, r.URL.Path)
			http.Error(w, "Доступ временно заблокирован", http.StatusTooManyRequests)
			return
		}

		// ШАГ 3: Обновляем счётчики запросов
		count := incrementRequestCount(ip)

		// ШАГ 4: Проверяем лимит запросов
		if count > requestLimit {
			blockIP(ip)
			logSecurityEvent("RATE_LIMIT_EXCEEDED", ip, r.URL.Path)
			http.Error(w, "Слишком много запросов. Попробуйте позже.", http.StatusTooManyRequests)
			return
		}

		// ШАГ 5: Проверяем подозрительную активность
		if isSuspicious(ip, r.URL.Path) {
			blockIP(ip)
			logSecurityEvent("SUSPICIOUS_ACTIVITY", ip, r.URL.Path)
			http.Error(w, "Подозрительная активность обнаружена", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ

// Получаем реальный IP (учитывая прокси и Heroku)
func getIP(r *http.Request) string {
	// Сначала проверяем X-Forwarded-For (актуально для Heroku)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Берём первый IP из списка (наиболее удалённый)
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Если нет X-Forwarded-For, берём RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// Проверяем, является ли IP доверенным
func isTrusted(ip string) bool {
	for _, trusted := range trustedIPs {
		if ip == trusted {
			return true
		}
	}
	return false
}

// Увеличиваем счётчик запросов для IP
func incrementRequestCount(ip string) int {
	countMutex.Lock()
	defer countMutex.Unlock()

	// Инициализируем время первого запроса
	if _, exists := lastRequestTime[ip]; !exists {
		lastRequestTime[ip] = time.Now()
	}

	// Обновляем время последнего запроса
	lastRequestTime[ip] = time.Now()

	// Увеличиваем счётчик
	requestCounts[ip]++
	return requestCounts[ip]
}

// Проверяем, заблокирован ли IP
func isBlocked(ip string) bool {
	countMutex.Lock()
	defer countMutex.Unlock()

	blockTime, exists := blockedIPs[ip]
	if !exists {
		return false
	}

	// Проверяем, не истёк ли срок блокировки
	return time.Since(blockTime) < blockDuration
}

// Блокируем IP на определённое время
func blockIP(ip string) {
	countMutex.Lock()
	defer countMutex.Unlock()

	blockedIPs[ip] = time.Now()
}

// Проверяем подозрительную активность
func isSuspicious(ip string, path string) bool {
	countMutex.Lock()
	defer countMutex.Unlock()

	// Правило 1: Слишком частые запросы к одному endpoint
	if count, exists := requestCounts[ip]; exists && count > requestLimit*2 {
		return true
	}

	// Правило 2: Запросы к несуществующим endpoint'ам
	suspiciousPaths := []string{"/admin", "/wp-login.php", "/.env", "/backup"}
	for _, sp := range suspiciousPaths {
		if strings.Contains(path, sp) {
			return true
		}
	}

	return false
}

// Логируем события безопасности
func logSecurityEvent(eventType, ip, path string) {
	securityLogger.Printf("%s | IP: %s | PATH: %s", eventType, ip, path)
}

// Очищаем старые записи из счётчиков
func cleanRequestCounts() {
	for {
		time.Sleep(5 * time.Minute)

		countMutex.Lock()
		currentTime := time.Now()

		// Удаляем IP, которые не делали запросы больше 10 минут
		for ip := range requestCounts {
			if lastTime, exists := lastRequestTime[ip]; exists {
				if currentTime.Sub(lastTime) > 10*time.Minute {
					delete(requestCounts, ip)
					delete(lastRequestTime, ip)
				}
			}
		}

		// Очищаем список заблокированных IP
		for ip, blockTime := range blockedIPs {
			if currentTime.Sub(blockTime) > blockDuration {
				delete(blockedIPs, ip)
			}
		}

		countMutex.Unlock()
	}
}
