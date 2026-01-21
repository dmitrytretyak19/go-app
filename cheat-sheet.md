# Шпаргалка Go (2026)

## Структура функции обработки запроса
1. Проверка метода: if r.Method != http.MethodPost
2. Извлечение данных: 
   - Из URL: idStr := r.URL.Path[len("/goals/"):]
   - Из Body: json.NewDecoder(r.Body).Decode(&data)
3. Подключение к БД: pgx.Connect(...)
4. SQL-запрос: conn.Exec(...) или conn.Query(...)
5. Ответ: json.NewEncoder(w).Encode(data)

## Английские термины
- func = функция
- var = переменная  
- err = ошибка
- Body = тело запроса
- Path = путь в URL