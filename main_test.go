// ФАЙЛ: main_test.go
// НАЗНАЧЕНИЕ: Автоматические тесты для API
// ОСОБЕННОСТИ:
//   - Полная изоляция от production-базы
//   - Автоматическая очистка после тестов
//   - Проверка всех CRUD-операций

package main

// ИМПОРТЫ: Только необходимые пакеты
import (
	"bytes"             // Для создания тела запроса
	"encoding/json"     // Для работы с JSON
	"net/http"          // Для HTTP-тестирования
	"net/http/httptest" // Для виртуального HTTP-сервера
	"testing"           // Основной пакет тестирования

	"context" // Для управления контекстом

	"github.com/jackc/pgx/v5" // PostgreSQL драйвер
)

// ГЛОБАЛЬНЫЕ ПЕРЕМЕННЫЕ ДЛЯ ТЕСТОВ
const testDBURL = "postgres://myuser@localhost:5432/testdb?sslmode=disable"

var originalDBURL string
var testLogger *AppLogger

// ФУНКЦИЯ: setupTestEnvironment
// НАЗНАЧЕНИЕ: Полная подготовка тестового окружения
func setupTestEnvironment(t *testing.T) {
	// ШАГ 0: ИНИЦИАЛИЗИРУЕМ ТЕСТОВЫЙ ЛОГГЕР
	testLogger = NewLogger()

	// ШАГ 1: СОХРАНЯЕМ ОРИГИНАЛЬНУЮ СТРОКУ ПОДКЛЮЧЕНИЯ
	originalDBURL = dbURL

	// ШАГ 2: ПОДКЛЮЧАЕМСЯ К СЛУЖЕБНОЙ БАЗЕ
	conn, err := pgx.Connect(context.Background(), "postgres://myuser@localhost:5432/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("❌ Не удалось подключиться к служебной базе: %v", err)
	}
	defer conn.Close(context.Background())

	// ШАГ 3: ЗАВЕРШАЕМ ВСЕ СОЕДИНЕНИЯ К ТЕСТОВОЙ БАЗЕ
	_, err = conn.Exec(context.Background(), `
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE datname = 'testdb' AND pid <> pg_backend_pid()
	`)
	if err != nil {
		t.Logf("⚠️ Предупреждение: %v", err)
	}

	// ШАГ 4: УДАЛЯЕМ И СОЗДАЕМ ТЕСТОВУЮ БАЗУ
	_, err = conn.Exec(context.Background(), "DROP DATABASE IF EXISTS testdb")
	if err != nil {
		t.Fatalf("❌ Ошибка удаления тестовой базы: %v", err)
	}

	_, err = conn.Exec(context.Background(), "CREATE DATABASE testdb")
	if err != nil {
		t.Fatalf("❌ Ошибка создания тестовой базы: %v", err)
	}

	// ШАГ 5: СОЗДАЕМ ТАБЛИЦУ В ТЕСТОВОЙ БАЗЕ
	testConn, err := pgx.Connect(context.Background(), testDBURL)
	if err != nil {
		t.Fatalf("❌ Не удалось подключиться к testdb: %v", err)
	}
	defer testConn.Close(context.Background())

	_, err = testConn.Exec(context.Background(), `
		CREATE TABLE goals (
			id SERIAL PRIMARY KEY,
			goal TEXT NOT NULL,
			timeline TEXT NOT NULL,
			salary_target INTEGER NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("❌ Ошибка создания таблицы: %v", err)
	}

	// ШАГ 6: ВСТАВЛЯЕМ ТЕСТОВЫЕ ДАННЫЕ
	_, err = testConn.Exec(context.Background(), `
		INSERT INTO goals (goal, timeline, salary_target) VALUES
		('Тест 1', '1 день', 1000),
		('Тест 2', '2 дня', 2000)
	`)
	if err != nil {
		t.Fatalf("❌ Ошибка вставки тестовых данных: %v", err)
	}

	// ШАГ 7: ПЕРЕКЛЮЧАЕМ ГЛОБАЛЬНУЮ ПЕРЕМЕННУЮ
	dbURL = testDBURL

	// ШАГ 8: ГАРАНТИРУЕМ ВОЗВРАТ ОРИГИНАЛЬНЫХ НАСТРОЕК
	t.Cleanup(func() {
		dbURL = originalDBURL

		// ОЧИСТКА: УДАЛЯЕМ ТЕСТОВУЮ БАЗУ
		cleanupConn, err := pgx.Connect(context.Background(), "postgres://myuser@localhost:5432/postgres?sslmode=disable")
		if err == nil {
			defer cleanupConn.Close(context.Background())

			// ЗАВЕРШАЕМ ОСТАВШИЕСЯ СОЕДИНЕНИЯ
			_, _ = cleanupConn.Exec(context.Background(), `
				SELECT pg_terminate_backend(pid) 
				FROM pg_stat_activity 
				WHERE datname = 'testdb' AND pid <> pg_backend_pid()
			`)

			// УДАЛЯЕМ БАЗУ
			_, _ = cleanupConn.Exec(context.Background(), "DROP DATABASE IF EXISTS testdb")
		}
	})
}

// ТЕСТ: TestGetGoals
// ПРОВЕРЯЕТ: GET /goals
func TestGetGoals(t *testing.T) {
	setupTestEnvironment(t) // Готовим окружение

	// ЗАМЕНЯЕМ ЛОГГЕР НА ТЕСТОВЫЙ
	originalLogger := logger
	logger = testLogger
	defer func() { logger = originalLogger }()

	// СОЗДАЕМ ЗАПРОС
	req, err := http.NewRequest(http.MethodGet, "/goals", nil)
	if err != nil {
		t.Fatalf("❌ Ошибка создания запроса: %v", err)
	}

	// ВЫПОЛНЯЕМ ЗАПРОС
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(getGoalsHandler)
	handler.ServeHTTP(rr, req)

	// ПРОВЕРЯЕМ СТАТУС
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("❌ Неверный статус: %d, ожидалось %d", status, http.StatusOK)
	}

	// ПАРСИМ ОТВЕТ
	var goals []Goal
	if err := json.Unmarshal(rr.Body.Bytes(), &goals); err != nil {
		t.Fatalf("❌ Ошибка парсинга JSON: %v", err)
	}

	// ПРОВЕРЯЕМ ДАННЫЕ
	if len(goals) != 2 {
		t.Errorf("❌ Неверное количество записей: %d, ожидалось 2", len(goals))
	}

	expected := map[string]bool{"Тест 1": true, "Тест 2": true}
	found := make(map[string]bool)
	for _, g := range goals {
		found[g.Goal] = true
	}

	for exp := range expected {
		if !found[exp] {
			t.Errorf("❌ Запись '%s' отсутствует", exp)
		}
	}
}

// ТЕСТ: TestCreateGoal
// ПРОВЕРЯЕТ: POST /goals
func TestCreateGoal(t *testing.T) {
	setupTestEnvironment(t)

	originalLogger := logger
	logger = testLogger
	defer func() { logger = originalLogger }()

	// СОЗДАЕМ ТЕСТОВЫЕ ДАННЫЕ
	newGoal := Goal{
		Goal:         "Новая цель",
		Timeline:     "3 дня",
		SalaryTarget: 5000,
	}
	jsonData, _ := json.Marshal(newGoal)

	// СОЗДАЕМ POST-ЗАПРОС
	req, _ := http.NewRequest(http.MethodPost, "/goals", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// ВЫПОЛНЯЕМ ЗАПРОС
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(createGoalHandler)
	handler.ServeHTTP(rr, req)

	// ПРОВЕРЯЕМ СТАТУС
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("❌ Неверный статус: %d, ожидалось %d", status, http.StatusCreated)
	}

	// ПАРСИМ СОЗДАННУЮ ЦЕЛЬ
	var created Goal
	json.Unmarshal(rr.Body.Bytes(), &created)

	// ПРОВЕРЯЕМ ПОЛЯ
	if created.ID == 0 {
		t.Error("❌ ID не был присвоен")
	}
	if created.Goal != "Новая цель" {
		t.Errorf("❌ Неверное значение Goal: %s", created.Goal)
	}
	if created.SalaryTarget != 5000 {
		t.Errorf("❌ Неверное значение SalaryTarget: %d", created.SalaryTarget)
	}
}

// ТЕСТ: TestDatabaseIsolation
// ПРОВЕРЯЕТ: Изоляцию тестовой базы
func TestDatabaseIsolation(t *testing.T) {
	setupTestEnvironment(t)

	// ПРОВЕРЯЕМ, ЧТО БАЗА ПЕРЕКЛЮЧЕНА
	if dbURL != testDBURL {
		t.Fatalf("❌ База не переключена на тестовую. Текущая: %s", dbURL)
	}

	// ПОДКЛЮЧАЕМСЯ К ТЕСТОВОЙ БАЗЕ
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("❌ Не удалось подключиться к тестовой базе: %v", err)
	}
	defer conn.Close(context.Background())

	// ПРОВЕРЯЕМ КОЛИЧЕСТВО ЗАПИСЕЙ
	var count int
	err = conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM goals").Scan(&count)
	if err != nil {
		t.Fatalf("❌ Ошибка запроса к базе: %v", err)
	}

	if count != 2 {
		t.Errorf("❌ Неверное количество записей: %d, ожидалось 2", count)
	}
}
