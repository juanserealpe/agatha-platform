package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// TextArray es una columna text[] de PostgreSQL con valores de un enum (sin
// comas, comillas ni llaves: FUGA, MOVIMIENTO). Evita depender de lib/pq.
type TextArray []string

// Value escribe el literal de arreglo: {FUGA,MOVIMIENTO}.
func (a TextArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	for _, v := range a {
		if strings.ContainsAny(v, `{},"\ `) {
			return nil, fmt.Errorf("text[]: valor no admitido %q", v)
		}
	}
	return "{" + strings.Join(a, ",") + "}", nil
}

// Scan lee el literal de arreglo que devuelve PostgreSQL.
func (a *TextArray) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		*a = nil
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("text[]: tipo no soportado %T", src)
	}
	s = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "{"), "}")
	if s == "" {
		*a = TextArray{}
		return nil
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.Trim(p, `"`)
	}
	*a = parts
	return nil
}

// GormDataType es el tipo de la columna.
func (TextArray) GormDataType() string { return "text[]" }
