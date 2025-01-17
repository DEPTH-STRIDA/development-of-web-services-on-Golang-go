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
	db         *sql.DB
	tables     []string
	primaryKey map[string]string // tableName -> primaryKeyName
}

// omitempty - не будет сериализован в JSON, если значение пустое
type Response struct {
	Response interface{} `json:"response,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type ColumnInfo struct {
	Type     string
	Nullable bool
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	explorer := &DbExplorer{
		db:         db,
		primaryKey: make(map[string]string),
	}

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
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

	for _, tableName := range tables {
		colRows, err := db.Query("SHOW COLUMNS FROM " + tableName)
		if err != nil {
			return nil, err
		}

		for colRows.Next() {
			var field, typ, null, key, extra string
			var def sql.NullString
			if err := colRows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
				colRows.Close()
				return nil, err
			}
			if key == "PRI" {
				explorer.primaryKey[tableName] = field
			}
		}
		colRows.Close()
		explorer.tables = append(explorer.tables, tableName)
	}

	return explorer, nil
}

func (explorer *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	switch r.Method {
	case http.MethodGet:
		if path == "" {
			explorer.handleTablesList(w, r)
			return
		}
		if len(parts) == 1 {
			explorer.handleTableRecords(w, r, parts[0])
			return
		}
		if len(parts) == 2 {
			explorer.handleRecord(w, r, parts[0], parts[1])
			return
		}
	case http.MethodPut:
		if len(parts) == 1 {
			explorer.handleCreate(w, r, parts[0])
			return
		}
	case http.MethodPost:
		if len(parts) == 2 {
			explorer.handleUpdate(w, r, parts[0], parts[1])
			return
		}
	case http.MethodDelete:
		if len(parts) == 2 {
			explorer.handleDelete(w, r, parts[0], parts[1])
			return
		}
	}

	http.Error(w, `{"error": "unknown method"}`, http.StatusNotFound)
}

func (explorer *DbExplorer) handleTablesList(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Response: map[string]interface{}{
			"tables": explorer.tables,
		},
	}
	json.NewEncoder(w).Encode(response)
}

func (explorer *DbExplorer) handleTableRecords(w http.ResponseWriter, r *http.Request, table string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	limit := 5
	offset := 0

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

	query := fmt.Sprintf("SELECT * FROM `%s` LIMIT ? OFFSET ?", table)
	rows, err := explorer.db.Query(query, limit, offset)
	if err != nil {
		http.Error(w, `{"error": "db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	records := make([]map[string]interface{}, 0)
	for rows.Next() {
		record, err := explorer.rowToMap(rows)
		if err != nil {
			http.Error(w, `{"error": "scan error"}`, http.StatusInternalServerError)
			return
		}
		records = append(records, record)
	}

	json.NewEncoder(w).Encode(Response{
		Response: map[string]interface{}{"records": records},
	})
}

func (explorer *DbExplorer) tableExists(table string) bool {
	for _, t := range explorer.tables {
		if t == table {
			return true
		}
	}
	return false
}

func (explorer *DbExplorer) handleRecord(w http.ResponseWriter, r *http.Request, table, id string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `%s` = ?", table, explorer.primaryKey[table])
	rows, err := explorer.db.Query(query, id)
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

func (explorer *DbExplorer) getColumnTypes(table string) (map[string]ColumnInfo, error) {
	colsRows, err := explorer.db.Query("SHOW COLUMNS FROM `" + table + "`")
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

func (explorer *DbExplorer) validateValue(value interface{}, colInfo ColumnInfo, field string) error {
	if value == nil {
		if !colInfo.Nullable {
			return fmt.Errorf("field %s have invalid type", field)
		}
		return nil
	}

	switch {
	case strings.Contains(colInfo.Type, "int"):
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("field %s have invalid type", field)
		}
	case strings.Contains(colInfo.Type, "varchar") || strings.Contains(colInfo.Type, "text") || strings.Contains(colInfo.Type, "char"):
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %s have invalid type", field)
		}
	}
	return nil
}

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

		if err := explorer.validateValue(value, info, field); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err), http.StatusBadRequest)
			return
		}

		columns = append(columns, field)
		values = append(values, value)
		placeholders = append(placeholders, "?")
	}

	query := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)",
		table,
		"`"+strings.Join(columns, "`, `")+"`",
		strings.Join(placeholders, ", "))

	result, err := explorer.db.Exec(query, values...)
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

	query := fmt.Sprintf("UPDATE `%s` SET %s WHERE `%s` = ?",
		table,
		strings.Join(sets, ", "),
		explorer.primaryKey[table])

	result, err := explorer.db.Exec(query, values...)
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

func (explorer *DbExplorer) handleDelete(w http.ResponseWriter, r *http.Request, table, id string) {
	if !explorer.tableExists(table) {
		http.Error(w, `{"error": "unknown table"}`, http.StatusNotFound)
		return
	}

	query := fmt.Sprintf("DELETE FROM `%s` WHERE `%s` = ?", table, explorer.primaryKey[table])
	result, err := explorer.db.Exec(query, id)
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

// Вспомогательная функция для преобразования результата запроса в map
func (explorer *DbExplorer) rowToMap(rows *sql.Rows) (map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, err
	}

	record := make(map[string]interface{})
	for i, col := range columns {
		var v interface{}
		val := values[i]
		if b, ok := val.([]byte); ok {
			v = string(b)
		} else {
			v = val
		}
		record[col] = v
	}

	return record, nil
}
