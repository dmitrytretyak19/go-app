// ФАЙЛ: handlers.go
// НАЗНАЧЕНИЕ: Обработчики HTTP-запросов для CRUD-операций
// ОСОБЕННОСТИ:
//   - Использование глобальной переменной dbURL для переключения баз
//   - Таймауты подключения к БД
//   - Полное логирование всех этапов

package main

// ИМПОРТЫ: Все необходимые пакеты
import (
	"context"       // Для контекста с таймаутами
	"encoding/json" // Для работы с JSON
	"net/http"      // Для HTTP-обработки
	"strconv"       // Для преобразования ID (используется в update/delete)
	"time"          // Для работы со временем (поле created_at)

	"github.com/jackc/pgx/v5" // PostgreSQL драйвер
)

// СТРУКТУРА ДАННЫХ ЦЕЛИ
// Соответствует таблице в базе данных
type Goal struct {
	ID           int       `json:"id"`                         // Уникальный ID (SERIAL в БД)
	Goal         string    `json:"goal"`                       // Текст цели
	Timeline     string    `json:"timeline"`                   // Срок выполнения
	SalaryTarget int       `json:"salary_target_rub_per_hour"` // Целевая зарплата
	CreatedAt    time.Time `json:"created_at"`                 // Время создания
}

// ОБРАБОТЧИК: GET /goals
// Получение всех целей из базы данных registeHandlers
func getGoalsHandler(w http.ResponseWriter, r *http.Request) {
	http.Handle("/test-panic", alertMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("Тестовая паника для проверки алертинга")
	})))
	// // ШАГ 1: ЛОГИРУЕМ НАЧАЛО ОБРАБОТКИ
	// Временный статус 0, будет обновлён позже
	logger.LogRequest(r.Method, r.URL.Path, 0)

	// ШАГ 2: ПОДКЛЮЧЕНИЕ К БД С ТАЙМАУТОМ 5 СЕКУНД
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel() // Гарантируем отмену контекста

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		// ЛОГИРУЕМ ОШИБКУ ПОДКЛЮЧЕНИЯ
		logger.LogError(err, "Подключение к БД в getGoalsHandler")
		// ОТПРАВЛЯЕМ ОТВЕТ 500
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		// ЛОГИРУЕМ ФАКТИЧЕСКИЙ СТАТУС
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx) // Гарантируем закрытие соединения

	// ШАГ 3: ВЫПОЛНЕНИЕ SQL-ЗАПРОСА
	// Сортируем по времени создания (старые записи первыми)
	rows, err := conn.Query(ctx,
		"SELECT id, goal, timeline, salary_target, created_at FROM goals ORDER BY created_at ASC")
	if err != nil {
		logger.LogError(err, "Ошибка выполнения SELECT в getGoalsHandler")
		http.Error(w, "Query error", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer rows.Close() // Закрываем курсор после использования

	// ШАГ 4: СБОР ДАННЫХ В СТРУКТУРЫ
	var goals []Goal
	for rows.Next() { // Перебираем все строки результата
		var g Goal
		// Сканируем данные из строки в структуру
		if err := rows.Scan(&g.ID, &g.Goal, &g.Timeline, &g.SalaryTarget, &g.CreatedAt); err != nil {
			logger.LogError(err, "Ошибка сканирования строки в getGoalsHandler")
			http.Error(w, "Scan error", http.StatusInternalServerError)
			logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
			return
		}
		goals = append(goals, g) // Добавляем в срез
	}

	// ШАГ 5: ОТПРАВКА УСПЕШНОГО ОТВЕТА
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(goals) // Кодируем срез в JSON
	// ЛОГИРУЕМ ФАКТИЧЕСКИЙ СТАТУС 200
	logger.LogRequest(r.Method, r.URL.Path, http.StatusOK)
}

// ОБРАБОТЧИК: POST /goals
// Создание новой цели в базе данных
func createGoalHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogRequest(r.Method, r.URL.Path, 0)

	// ШАГ 1: ПРОВЕРКА HTTP-МЕТОДА
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
		return
	}

	// ШАГ 2: ДЕКОДИРОВАНИЕ JSON ИЗ ТЕЛА ЗАПРОСА
	var newGoal Goal
	if err := json.NewDecoder(r.Body).Decode(&newGoal); err != nil {
		logger.LogError(err, "Ошибка декодирования JSON в createGoalHandler")
		http.Error(w, "Неверный JSON", http.StatusBadRequest)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusBadRequest)
		return
	}

	// ШАГ 3: ПОДКЛЮЧЕНИЕ К БД
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.LogError(err, "Подключение к БД в createGoalHandler")
		http.Error(w, "Ошибка подключения к БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// ШАГ 4: ВСТАВКА ЗАПИСИ В БАЗУ
	// NOW() автоматически устанавливает текущее время
	// RETURNING id возвращает сгенерированный ID
	query := `INSERT INTO goals (goal, timeline, salary_target, created_at) VALUES ($1, $2, $3, NOW()) RETURNING id`
	err = conn.QueryRow(ctx, query, newGoal.Goal, newGoal.Timeline, newGoal.SalaryTarget).Scan(&newGoal.ID)
	if err != nil {
		logger.LogError(err, "Ошибка вставки в БД в createGoalHandler")
		http.Error(w, "Ошибка записи в БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}

	// ШАГ 5: ОТПРАВКА СОЗДАННОЙ ЗАПИСИ
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated) // 201 Created
	json.NewEncoder(w).Encode(newGoal)
	logger.LogRequest(r.Method, r.URL.Path, http.StatusCreated)
}

// ОБРАБОТЧИК: PUT /goals/{id}
// Обновление существующей цели
func updateGoalHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogRequest(r.Method, r.URL.Path, 0)

	// ШАГ 1: ПРОВЕРКА HTTP-МЕТОДА
	if r.Method != http.MethodPut {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
		return
	}

	// ШАГ 2: ИЗВЛЕЧЕНИЕ ID ИЗ URL
	// Пример: /goals/11 → "11"
	idStr := r.URL.Path[len("/goals/"):]
	id, err := strconv.Atoi(idStr) // Преобразуем строку в число
	if err != nil {
		logger.LogError(err, "Неверный ID в updateGoalHandler")
		http.Error(w, "Неверный ID", http.StatusBadRequest)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusBadRequest)
		return
	}

	// ШАГ 3: ДЕКОДИРОВАНИЕ JSON
	var updatedGoal Goal
	if err := json.NewDecoder(r.Body).Decode(&updatedGoal); err != nil {
		logger.LogError(err, "Ошибка декодирования JSON в updateGoalHandler")
		http.Error(w, "Неверный JSON", http.StatusBadRequest)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusBadRequest)
		return
	}

	// ШАГ 4: ПОДКЛЮЧЕНИЕ К БД
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.LogError(err, "Подключение к БД в updateGoalHandler")
		http.Error(w, "Ошибка подключения к БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// ШАГ 5: ОБНОВЛЕНИЕ ЗАПИСИ
	// WHERE id = $4 использует параметризованный запрос для безопасности
	query := `UPDATE goals SET goal = $1, timeline = $2, salary_target = $3 WHERE id = $4`
	result, err := conn.Exec(ctx, query, updatedGoal.Goal, updatedGoal.Timeline, updatedGoal.SalaryTarget, id)
	if err != nil {
		logger.LogError(err, "Ошибка обновления в БД в updateGoalHandler")
		http.Error(w, "Ошибка обновления в БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}

	// ШАГ 6: ПРОВЕРКА, БЫЛА ЛИ ЗАПИСЬ НАЙДЕНА
	if result.RowsAffected() == 0 {
		errMsg := "Запись не найдена"
		logger.LogError(nil, errMsg) // Бизнес-ошибка (nil вместо err)
		http.Error(w, errMsg, http.StatusNotFound)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusNotFound)
		return
	}

	// ШАГ 7: ОТПРАВКА ОБНОВЛЁННОЙ ЗАПИСИ
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(updatedGoal)
	logger.LogRequest(r.Method, r.URL.Path, http.StatusOK)
}

// ОБРАБОТЧИК: DELETE /goals/{id}
// Удаление цели из базы данных
func deleteGoalHandler(w http.ResponseWriter, r *http.Request) {
	logger.LogRequest(r.Method, r.URL.Path, 0)

	// ШАГ 1: ПРОВЕРКА HTTP-МЕТОДА
	if r.Method != http.MethodDelete {
		http.Error(w, "Метод не разрешён", http.StatusMethodNotAllowed)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusMethodNotAllowed)
		return
	}

	// ШАГ 2: ИЗВЛЕЧЕНИЕ ID ИЗ URL
	idStr := r.URL.Path[len("/goals/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.LogError(err, "Неверный ID в deleteGoalHandler")
		http.Error(w, "Неверный ID", http.StatusBadRequest)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusBadRequest)
		return
	}

	// ШАГ 3: ПОДКЛЮЧЕНИЕ К БД
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		logger.LogError(err, "Подключение к БД в deleteGoalHandler")
		http.Error(w, "Ошибка подключения к БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}
	defer conn.Close(ctx)

	// ШАГ 4: УДАЛЕНИЕ ЗАПИСИ
	// Используем $1 для защиты от SQL-инъекций
	result, err := conn.Exec(ctx, "DELETE FROM goals WHERE id = $1", id)
	if err != nil {
		logger.LogError(err, "Ошибка удаления в БД в deleteGoalHandler")
		http.Error(w, "Ошибка удаления из БД", http.StatusInternalServerError)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusInternalServerError)
		return
	}

	// ШАГ 5: ПРОВЕРКА, БЫЛА ЛИ ЗАПИСЬ НАЙДЕНА
	if result.RowsAffected() == 0 {
		errMsg := "Запись не найдена"
		logger.LogError(nil, errMsg)
		http.Error(w, errMsg, http.StatusNotFound)
		logger.LogRequest(r.Method, r.URL.Path, http.StatusNotFound)
		return
	}

	// ШАГ 6: УСПЕШНОЕ УДАЛЕНИЕ
	// 204 No Content — стандарт для успешного удаления без тела ответа
	w.WriteHeader(http.StatusNoContent)
	logger.LogRequest(r.Method, r.URL.Path, http.StatusNoContent)
}
