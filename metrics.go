// ФАЙЛ: metrics.go
// НАЗНАЧЕНИЕ: Сбор метрик для мониторинга
// ВАЖНО: Этот файл нужно создать в корне проекта

package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ДЛЯ МЕТРИК
var (
	// СЧЁТЧИК ЗАПРОСОВ
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Общее количество HTTP запросов",
		},
		[]string{"method", "endpoint", "status"},
	)

	// ЗАМЕР ВРЕМЕНИ ОБРАБОТКИ
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Время обработки запросов в секундах",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"method", "endpoint"},
	)
)

// ИНИЦИАЛИЗАЦИЯ МЕТРИК
func initMetrics() {
	prometheus.MustRegister(requestCount)
	prometheus.MustRegister(requestDuration)
	log.Println("✅ Метрики зарегистрированы в Prometheus")
}

// MIDDLEWARE ДЛЯ СБОРА МЕТРИК
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Выполняем основной обработчик
		next.ServeHTTP(w, r)

		// Считаем время выполнения
		duration := time.Since(start).Seconds()

		// Логируем для отладки
		logger.InfoLogger.Printf("📊 METRIC: %s %s | %.3f сек", r.Method, r.URL.Path, duration)

		// Обновляем счётчики
		requestCount.WithLabelValues(r.Method, r.URL.Path, "200").Inc()
		requestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

// РЕГИСТРАЦИЯ ENDPOINT ДЛЯ PROMETHEUS
func registerMetricsEndpoint() {
	http.Handle("/metrics", promhttp.Handler())
	logger.InfoLogger.Println("✅ Endpoint /metrics зарегистрирован")
}
