package sqlite

import (
	"database/sql"
	"encoding/json"
)

func encode(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func decode[T any](body string) (T, error) {
	var value T
	err := json.Unmarshal([]byte(body), &value)
	return value, err
}

func decodeRows[T any](rows *sql.Rows) ([]T, error) {
	defer rows.Close()
	values := make([]T, 0)
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		value, err := decode[T](body)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
