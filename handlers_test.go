package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestMain(m *testing.M) {
	// Инициализируем логгер
	logger = NewLogger()

	// ЯВНО устанавливаем тестовую БД для тестов
	dbURL = "postgres://myuser:mypass@localhost:5432/testdb?sslmode=disable"

	// Подключаемся к БД
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.LogError(err, "❌ Не удалось подключиться к тестовой БД")
		os.Exit(1)
	}
	defer conn.Close(ctx)

	logger.InfoLogger.Println("✅ Тестовая БД подключена")

	// Удаляем таблицу если она существует
	_, _ = conn.Exec(ctx, "DROP TABLE IF EXISTS goals")

	// Создаем таблицу goals с ТОЧНОЙ структурой из основного приложения
	_, err = conn.Exec(ctx, `
	CREATE TABLE goals (
		id SERIAL PRIMARY KEY,
		goal TEXT NOT NULL,
		timeline TEXT NOT NULL,
		salary_target INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	)
	`)
	if err != nil {
		logger.LogError(err, "❌ Не удалось создать таблицу goals")
		os.Exit(1)
	}
	logger.InfoLogger.Println("✅ Таблица goals создана с точной структурой из приложения")

	// Запускаем тесты
	code := m.Run()

	// Очищаем данные после тестов
	_, _ = conn.Exec(ctx, "TRUNCATE TABLE goals RESTART IDENTITY")

	os.Exit(code)
}

// ТЕСТ: Создание цели
func TestCreateGoal(t *testing.T) {
	goal := Goal{
		Goal:         "Test Goal",
		Timeline:     "Test Timeline",
		SalaryTarget: 1000,
	}
	jsonData, _ := json.Marshal(goal)

	req := httptest.NewRequest("POST", "/goals", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	createGoalHandler(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, recorder.Code)
	}
}

// ТЕСТ: Получение целей
func TestGetGoals(t *testing.T) {
	req := httptest.NewRequest("GET", "/goals", nil)
	recorder := httptest.NewRecorder()

	getGoalsHandler(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

// ТЕСТ: Неверный JSON
func TestInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/goals", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	createGoalHandler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

// ТЕСТ: Обновление несуществующей цели
func TestUpdateNonExistentGoal(t *testing.T) {
	goal := Goal{
		Goal:         "Updated Goal",
		Timeline:     "Updated Timeline",
		SalaryTarget: 2000,
	}
	jsonData, _ := json.Marshal(goal)

	req := httptest.NewRequest("PUT", "/goals/999999", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	updateGoalHandler(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

// ТЕСТ: Удаление несуществующей цели
func TestDeleteNonExistentGoal(t *testing.T) {
	req := httptest.NewRequest("DELETE", "/goals/999999", nil)
	recorder := httptest.NewRecorder()

	deleteGoalHandler(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

// ТЕСТ: Успешное удаление существующей цели
func TestDeleteExistingGoal(t *testing.T) {
	// Сначала создаем цель
	goal := Goal{
		Goal:         "Goal to delete",
		Timeline:     "Timeline to delete",
		SalaryTarget: 3000,
	}
	jsonData, _ := json.Marshal(goal)

	createReq := httptest.NewRequest("POST", "/goals", bytes.NewBuffer(jsonData))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	createGoalHandler(createRecorder, createReq)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("Failed to create goal for deletion test")
	}

	// Получаем ID созданной цели из ответа
	var createdGoal Goal
	err := json.Unmarshal(createRecorder.Body.Bytes(), &createdGoal)
	if err != nil {
		t.Fatalf("Failed to parse created goal: %v", err)
	}

	// Теперь удаляем цель
	deleteReq := httptest.NewRequest("DELETE", "/goals/"+string(rune('0'+createdGoal.ID)), nil)
	deleteRecorder := httptest.NewRecorder()
	deleteGoalHandler(deleteRecorder, deleteReq)

	// ИСПРАВЛЕНО: Ожидаем статус 204 (No Content) вместо 200
	if deleteRecorder.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, deleteRecorder.Code)
	}
}

// ТЕСТ: Успешное обновление существующей цели
func TestUpdateExistingGoal(t *testing.T) {
	// Сначала создаем цель
	goal := Goal{
		Goal:         "Original Goal",
		Timeline:     "Original Timeline",
		SalaryTarget: 4000,
	}
	jsonData, _ := json.Marshal(goal)

	createReq := httptest.NewRequest("POST", "/goals", bytes.NewBuffer(jsonData))
	createReq.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	createGoalHandler(createRecorder, createReq)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("Failed to create goal for update test")
	}

	// Получаем ID созданной цели
	var createdGoal Goal
	err := json.Unmarshal(createRecorder.Body.Bytes(), &createdGoal)
	if err != nil {
		t.Fatalf("Failed to parse created goal: %v", err)
	}

	// Теперь обновляем цель
	updatedGoal := Goal{
		Goal:         "Updated Goal",
		Timeline:     "Updated Timeline",
		SalaryTarget: 5000,
	}
	updateData, _ := json.Marshal(updatedGoal)

	updateReq := httptest.NewRequest("PUT", "/goals/"+string(rune('0'+createdGoal.ID)), bytes.NewBuffer(updateData))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	updateGoalHandler(updateRecorder, updateReq)

	if updateRecorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, updateRecorder.Code)
	}
}
