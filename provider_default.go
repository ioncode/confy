package confy

// DefaultProvider извлекает значения из тегов `default` и инициализирует ими структуру конфигурации.
type DefaultProvider struct{}

// Bind накладывает дефолтные текстовые значения тегов на рантайм-поля структуры.
func (p *DefaultProvider) Bind(fields []FieldInstance) error {
	for _, f := range fields {
		if f.Meta.Default != "" {
			if err := setPrimitiveValue(f.Value, f.Meta.Default); err != nil {
				return &ErrParsingValue{Source: "tag default", TargetField: f.Meta.FieldName, Err: err}
			}
		}
	}
	return nil
}
