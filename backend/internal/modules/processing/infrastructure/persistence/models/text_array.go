package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// TextArray mapea una columna TEXT[] de PostgreSQL. Pensado para identificadores
// simples (flags en mayúsculas): no escapa comas, comillas ni llaves.
// Copia deliberada del de Baseline: los módulos no comparten modelos.
type TextArray []string

// Value serializa al literal de arreglo de PostgreSQL: {A,B}.
func (a TextArray) Value() (driver.Value, error) {
	return "{" + strings.Join(a, ",") + "}", nil
}

// Scan parsea el literal {A,B} (también {"A","B"}).
func (a *TextArray) Scan(src any) error {
	var raw string
	switch v := src.(type) {
	case nil:
		*a = TextArray{}
		return nil
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("TextArray: tipo no soportado %T", src)
	}

	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}")
	if raw == "" {
		*a = TextArray{}
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make(TextArray, len(parts))
	for i, p := range parts {
		out[i] = strings.Trim(p, `"`)
	}
	*a = out

	return nil
}
