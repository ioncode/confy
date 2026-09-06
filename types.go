package confy

import (
	"reflect"
	"strconv"
	"strings"
	"time"
)

// stringSliceValue реализует интерфейс flag.Value для поддержки парсинга []string флагов через запятую.
type stringSliceValue struct {
	value *[]string
}

// String возвращает строковое представление среза строк через запятую.
func (s *stringSliceValue) String() string {
	if s.value == nil {
		return ""
	}
	return strings.Join(*s.value, ",")
}

// Set парсит входящую строку, разделяя её по запятым и удаляя лишние пробелы.
func (s *stringSliceValue) Set(val string) error {
	if val == "" {
		*s.value = []string{}
		return nil
	}
	parts := strings.Split(val, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	*s.value = parts
	return nil
}

// setPrimitiveValue приводит строковое значение к целевому примитивному типу поля структуры.
func setPrimitiveValue(field reflect.Value, val string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(val)
	case reflect.Uint16:
		p, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return err
		}
		field.SetUint(p)
	case reflect.Int, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(d))
		} else {
			p, err := strconv.Atoi(val)
			if err != nil {
				return err
			}
			field.SetInt(int64(p))
		}
	case reflect.Bool:
		p, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		field.SetBool(p)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(val, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			field.Set(reflect.ValueOf(parts))
		}
	}
	return nil
}
