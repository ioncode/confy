// Package confy предоставляет легковесный, рекурсивный инструмент для декларативной
// конфигурации Go-приложений с помощью структурных тегов на базе чистой архитектуры.
//
// Библиотека объединяет значения из трех источников в порядке возрастания приоритета:
//  1. Тег `default:"значение"` (самый низкий приоритет)
//  2. Флаги запуска CLI (`-flag`)
//  3. Переменные окружения (`ENV_VAR`) (самый высокий приоритет)
//
// Модуль полностью потокобезопасен, кэширует схему рефлексии и переиспользует память в рантайме.
package confy

import (
	"fmt"
	"reflect"
)

// Константы для названий структурных тегов во избежание опечаток в коде парсеров.
const (
	TagFlag     = "flag"
	TagEnv      = "env"
	TagDefault  = "default"
	TagRequired = "required"
	TagUsage    = "usage"
)

// FieldMeta описывает абстрактные метаданные одного поля конфигурации,
// очищенные от специфики конкретных инфраструктурных провайдеров.
type FieldMeta struct {
	FieldName  string // Имя поля в структуре (например, "Port")
	FlagName   string // Имя соответствующего флага CLI (например, "db-port")
	EnvName    string // Полное имя переменной окружения (например, "APP_DB_PORT")
	Default    string // Значение по умолчанию из тега string
	Usage      string // Описание поля для генерации автоматической справки
	Required   bool   // Флаг обязательности заполнения поля
	FieldIndex []int  // Индексы для быстрого доступа к полю структуры без повторной рефлексии
}

// ConfigProvider определяет контракт для источников данных (флаги, env, конфигурационные файлы).
type ConfigProvider interface {
	// Bind принимает срез связанных рантайм-значений полей для наложения данных источника.
	Bind(fields []FieldInstance) error
}

// FieldInstance связывает метаданные поля с его конкретным рантайм-значением reflect.Value.
type FieldInstance struct {
	Meta  FieldMeta
	Value reflect.Value
}

// Validator определяет интерфейс для кастомной валидации доменной модели пользователем.
// Если структура конфигурации его реализует, метод вызывается автоматически в конце работы Load.
type Validator interface {
	Validate() error
}

// ErrInvalidConfigTarget возвращается, если в метод Load передан не указатель на структуру.
type ErrInvalidConfigTarget struct {
	Kind reflect.Kind
}

// Error возвращает строковое представление ошибки ErrInvalidConfigTarget.
func (e *ErrInvalidConfigTarget) Error() string {
	return fmt.Sprintf("cfg должен быть указателем на структуру, получено: %s", e.Kind)
}

// ErrRequiredFieldMissing возвращается, если обязательное поле с тегом required:"true"
// осталось неинициализированным после применения всех провайдеров данных.
type ErrRequiredFieldMissing struct {
	FieldName string
}

// Error возвращает строковое представление ошибки ErrRequiredFieldMissing.
func (e *ErrRequiredFieldMissing) Error() string {
	return fmt.Sprintf("поле %s является обязательным (required)", e.FieldName)
}

// ErrParsingValue возвращается при ошибках конвертации строковых данных в типы полей структуры.
type ErrParsingValue struct {
	Source      string
	TargetField string
	Err         error
}

// Error возвращает строковое представление ошибки ErrParsingValue.
func (e *ErrParsingValue) Error() string {
	return fmt.Sprintf("ошибка парсинга значения из %s для поля %s: %v", e.Source, e.TargetField, e.Err)
}
