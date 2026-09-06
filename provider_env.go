package confy

import "os"

// EnvProvider отвечает за извлечение конфигурационных данных из переменных окружения (Environment Variables).
type EnvProvider struct{}

// Bind считывает данные из ОС и перезаписывает рантайм-значения полей структуры, если переменные заданы.
func (p *EnvProvider) Bind(fields []FieldInstance) error {
	for _, f := range fields {
		if f.Meta.EnvName == "" {
			continue
		}
		if val := os.Getenv(f.Meta.EnvName); val != "" {
			if err := setPrimitiveValue(f.Value, val); err != nil {
				return &ErrParsingValue{Source: "environment variable", TargetField: f.Meta.FieldName, Err: err}
			}
		}
	}
	return nil
}
