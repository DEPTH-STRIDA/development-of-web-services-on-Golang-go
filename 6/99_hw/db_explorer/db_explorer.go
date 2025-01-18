package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// экземпляр структуры хранится только внутри функции NewDbExplorer,
// таким образом и выполнено замыкание
type DbExplorer struct {
	db *sql.DB
	// 1. запрос всех таблиц можно кешировать. Они не меняются по тз.
	// 2. primaryKey необходимо для выполнения запроса на получение записи по id (where)
	tables     []string
	primaryKey map[string]string // tableName -> primaryKeyName
}

// Response универсальный ответ, который будет маршалиться для ответа в тела ответов.
// omitempty - не будет сериализован в JSON, если значение пустое
type Response struct {
	Response interface{} `json:"response,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// ColumnInfo хранит метаданные о колонке таблицы:
// 1. Type - тип данных колонки (string, int, float и т.д.) для валидации входящих значений
// 2. Nullable - может ли поле быть null, используется при:
//   - валидации входящих данных
//   - автозаполнении NOT NULL полей пустыми значениями при создании записи
type ColumnInfo struct {
	Type     string
	Nullable bool
}

// Конструктор DbExplorer
func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	explorer := &DbExplorer{
		db:         db,
		primaryKey: make(map[string]string),
	}

	// Первоначальный запрос для кеширования данных о таблицах и их первичных ключах
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	// Обязательно закрываем строку, чтобы не было утечки ресурсов
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}
	rows.Close()

	// Для каждой таблицы получаем информацию о её колонках
	for _, tableName := range tables {
		// Запрашиваем структуру таблицы
		colRows, err := db.Query(fmt.Sprintf("SHOW COLUMNS FROM `%s`", tableName))
		if err != nil {
			return nil, err
		}

		// Обрабатываем каждую колонку таблицы
		for colRows.Next() {
			// field - имя колонки
			// typ - тип данных
			// null - может ли быть NULL
			// key - тип ключа (PRI для первичного)
			// def - значение по умолчанию
			// extra - дополнительные свойства
			var field, typ, null, key, extra string
			var def sql.NullString
			if err := colRows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
				colRows.Close()
				return nil, err
			}
			// Если это первичный ключ - сохраняем его имя для данной таблицы
			if key == "PRI" {
				explorer.primaryKey[tableName] = field
			}
		}
		colRows.Close()
		// Добавляем таблицу в список известных таблиц
		explorer.tables = append(explorer.tables, tableName)
	}

	return explorer, nil
}

// ServeHTTP обрабатывает запросы к сервису
// Типичный http.Handler
func (explorer *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// path - путь запроса, например /table1/123
	path := strings.Trim(r.URL.Path, "/")
	// parts - массив путей, например ["table1", "123"]
	parts := strings.Split(path, "/")

	n := 0
	// Особый случай - корневой путь "/".
	// "/" len = 1
	if path == "" {
		n = 0
	} else {
		// n - количество сегментов пути
		n = len(parts)
	}

	switch r.Method {
	///////////////////////////////////////////////////////////////
	// GET
	case http.MethodGet:
		switch n {
		case 0: // n = 0
			explorer.handleTablesList(w, r)
			return
		case 1: // n = 1
			explorer.handleTableRecords(w, r, parts[0])
			return
		case 2: // n = 2
			explorer.handleRecord(w, r, parts[0], parts[1])
			return
		}
	////////////////////////////////////////////////////////////////
	// PUT
	case http.MethodPut:
		switch n {
		case 1: // n = 1
			explorer.handleCreate(w, r, parts[0])
			return
		}
	////////////////////////////////////////////////////////////////
	// POST
	case http.MethodPost:
		switch n {
		case 2: // n = 2
			explorer.handleUpdate(w, r, parts[0], parts[1])
			return
		}
	////////////////////////////////////////////////////////////////
	// DELETE
	case http.MethodDelete:
		switch n {
		case 2: // n = 2
			explorer.handleDelete(w, r, parts[0], parts[1])
			return
		}
	}

	http.Error(w, `{"error": "unknown method"}`, http.StatusNotFound)
}

// handleTablesList обрабатывает запрос на получение списка всех таблиц
func (explorer *DbExplorer) handleTablesList(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Response: map[string]interface{}{
			"tables": explorer.tables,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleTableRecords обрабатывает запрос на получение всех записей таблицы
func (explorer *DbExplorer) handleTableRecords(w http.ResponseWriter, r *http.Request, table string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	// Значения по умолчанию для limit и offset
	limit := 5
	offset := 0

	// Получаем параметры limit и offset из запроса
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	// Формируем запрос на получение записей таблицы
	query := "SELECT * FROM ? LIMIT ? OFFSET ?"
	rows, err := explorer.db.Query(query, table, limit, offset)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Считываем все записи из базы
	records := make([]map[string]interface{}, 0)
	// Считываем каждую запись
	for rows.Next() {
		// Считываем запись в структуру
		record, err := explorer.rowToMap(rows)
		if err != nil {
			http.Error(w, `{"error": "scan error"}`, http.StatusInternalServerError)
			return
		}
		// Добавляем запись в список
		records = append(records, record)
	}

	// Отправляем ответ
	json.NewEncoder(w).Encode(Response{
		Response: map[string]interface{}{"records": records},
	})
}

// handleRecord обрабатывает запрос на получение записи по id
func (explorer *DbExplorer) handleRecord(w http.ResponseWriter, r *http.Request, table, id string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	// Формируем запрос на получение записи по id
	query := "SELECT * FROM ? WHERE ? = ?"
	rows, err := explorer.db.Query(query, table, explorer.primaryKey[table], id)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if !rows.Next() {
		http.Error(w, `{"error": "record not found"}`, http.StatusNotFound)
		return
	}

	record, err := explorer.rowToMap(rows)
	if err != nil {
		http.Error(w, `{"error": "scan error"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(Response{
		Response: map[string]interface{}{"record": record},
	})
}

// handleCreate обрабатывает запрос на создание новой записи в таблице
func (explorer *DbExplorer) handleCreate(w http.ResponseWriter, r *http.Request, table string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	columnTypes, err := explorer.getColumnTypes(table)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}

	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, `{"error": "bad request"}`, http.StatusBadRequest)
		return
	}

	columns := make([]string, 0)
	values := make([]interface{}, 0)
	placeholders := make([]string, 0)

	// Проверяем все колонки
	for field, info := range columnTypes {
		if field == explorer.primaryKey[table] {
			continue
		}

		value, exists := requestData[field]
		if !exists {
			// Если поле не передано и оно NOT NULL без default value
			if !info.Nullable {
				value = "" // для строк пустая строка, для int можно 0
			}
		}

		// Проверяем значение на соответствие типу колонки
		if err := explorer.validateValue(value, info, field); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadRequest)
			return
		}

		columns = append(columns, field)
		values = append(values, value)
		placeholders = append(placeholders, "?")
	}

	// Формируем запрос на создание новой записи в таблице
	query := "INSERT INTO ? (?) VALUES (?)"

	result, err := explorer.db.Exec(query, table, columns, placeholders)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	response := Response{
		Response: map[string]interface{}{
			explorer.primaryKey[table]: id,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleUpdate обрабатывает запрос на обновление записи в таблице
func (explorer *DbExplorer) handleUpdate(w http.ResponseWriter, r *http.Request, table, id string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	columnTypes, err := explorer.getColumnTypes(table)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}

	var requestData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, `{"error": "bad request"}`, http.StatusBadRequest)
		return
	}

	// Проверяем попытку обновить primary key
	primaryKey := explorer.primaryKey[table]
	if _, ok := requestData[primaryKey]; ok {
		http.Error(w, fmt.Sprintf(`{"error": "field %s have invalid type"}`, primaryKey), http.StatusBadRequest)
		return
	}

	sets := make([]string, 0)
	values := make([]interface{}, 0)

	for key, value := range requestData {
		colInfo, ok := columnTypes[key]
		if !ok {
			continue
		}

		if err := explorer.validateValue(value, colInfo, key); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadRequest)
			return
		}

		// Формируем запрос на обновление записи в таблице
		sets = append(sets, fmt.Sprintf("`%s` = ?", key))
		values = append(values, value)
	}
	values = append(values, id)

	if len(sets) == 0 {
		response := Response{
			Response: map[string]interface{}{
				"updated": 0,
			},
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Формируем запрос на обновление записи в таблице
	query := "UPDATE ? SET ? WHERE ? = ?"

	result, err := explorer.db.Exec(query, table, sets, explorer.primaryKey[table], id)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	response := Response{
		Response: map[string]interface{}{
			"updated": affected,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleDelete обрабатывает запрос на удаление записи из таблицы
func (explorer *DbExplorer) handleDelete(w http.ResponseWriter, r *http.Request, table, id string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	// Формируем запрос на удаление записи из таблицы
	query := "DELETE FROM ? WHERE ? = ?"
	result, err := explorer.db.Exec(query, table, explorer.primaryKey[table], id)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	response := Response{
		Response: map[string]interface{}{
			"deleted": affected,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// validateValue проверяет значение на соответствие типу колонки
func (explorer *DbExplorer) validateValue(value interface{}, colInfo ColumnInfo, field string) error {
	if value == nil {
		if !colInfo.Nullable {
			return fmt.Errorf("field %s have invalid type", field)
		}
		return nil
	}

	switch {
	case strings.Contains(colInfo.Type, "int"):
		// Проверяем сначала float64 (из JSON)
		if floatVal, ok := value.(float64); ok {
			// Проверяем, что число целое
			if floatVal == float64(int64(floatVal)) {
				return nil
			}
		}
		// Проверяем int (для других случаев)
		if _, ok := value.(int); ok {
			return nil
		}
		return fmt.Errorf("field %s have invalid type", field)

	case strings.Contains(colInfo.Type, "varchar") ||
		strings.Contains(colInfo.Type, "text") ||
		strings.Contains(colInfo.Type, "char"):
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s have invalid type", field)
		}
	}
	return nil
}

// tableExists проверяет, существует ли таблица в списке известных таблиц.
// Проверка ведется по кешу сохраненному в структуре DbExplorer
func (explorer *DbExplorer) tableExists(table string) bool {
	for _, t := range explorer.tables {
		if t == table {
			return true
		}
	}
	return false
}

// getColumnTypes получает типы колонок таблицы
func (explorer *DbExplorer) getColumnTypes(table string) (map[string]ColumnInfo, error) {
	query := fmt.Sprintf("SHOW COLUMNS FROM `%s`", table)
	colsRows, err := explorer.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer colsRows.Close()

	columnTypes := make(map[string]ColumnInfo)
	for colsRows.Next() {
		var field, typ, null, key, extra string
		var def sql.NullString
		if err := colsRows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
			return nil, err
		}
		columnTypes[field] = ColumnInfo{
			Type:     typ,
			Nullable: null == "YES",
		}
	}
	return columnTypes, nil
}

// rowToMap преобразует строку результата sql.Rows в map[string]interface{}
// Используется для формирования JSON-ответа
// Ключи map - имена колонок, значения - данные из базы
func (explorer *DbExplorer) rowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	// Получаем список имен колонок из результата запроса
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Создаем слайсы для значений и указателей на них
	// КЛЮЧ valuePtrs (pointers) хранит указатели на них для Scan
	// ЗНАЧЕНИЕ values хранит сами значения
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// Сканируем строку результата в массив values через указатели valuePtrs
	// Записывает в ключи имена колонок
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	// Формируем map из имен колонок и их значений
	record := make(map[string]interface{})
	for i, col := range columns {
		var v interface{}
		val := values[i]
		// Особая обработка для []byte - конвертируем в string
		// MySQL возвращает строковые типы как []byte
		if b, ok := val.([]byte); ok {
			v = string(b)
		} else {
			v = val
		}
		record[col] = v
	}

	return record, nil
}
