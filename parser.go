package confy

import (
	"reflect"
	"strings"
	"sync"
)

// cachedSchema инкапсулирует вычисленные метаданные и синхронизацию флагов для конкретного типа.
type cachedSchema struct {
	meta []FieldMeta
}

// schemaCache — глобальный потокобезопасный кэш схем типов.
// Предотвращает повторный разбор тегов и путей полей структуры при многократных вызовах Load.
var schemaCache sync.Map

// instancePool — пул объектов для повторного использования рантайм-срезов FieldInstance.
// Позволяет полностью исключить аллокации памяти в куче при частых перезагрузках или в бенчмарках.
var instancePool = sync.Pool{
	New: func() any {
		return &[]FieldInstance{}
	},
}

// Loader является оркестратором (слой Usecase), координирующим сбор метаданных и применение провайдеров.
type Loader struct {
	envPrefix    string
	args         []string
	flagProvider *FlagProvider
	onceFlags    sync.Once
}

// New инициализирует и возвращает экземпляр Loader для конкретного набора параметров сборки.
func New(envPrefix string, args []string) *Loader {
	return &Loader{
		envPrefix:    envPrefix,
		args:         args,
		flagProvider: NewFlagProvider(args),
	}
}

// Load выполняет сборку конфигурации из трех источников (дефолты, флаги, ENV) для переданной структуры.
//
// Метод оптимизирован с помощью глобального кэша и пула объектов, обеспечивая O(1) рантайм-доступ
// к полям конфигурации без генерации лишнего мусора в памяти (Zero-allocation).
func (l *Loader) Load(cfg any) error {
	t := reflect.TypeOf(cfg)
	if t == nil || t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct {
		return &ErrInvalidConfigTarget{Kind: reflect.ValueOf(cfg).Kind()}
	}

	v := reflect.ValueOf(cfg)
	structVal := v.Elem()
	structType := t.Elem()

	// 1. Получаем неизменяемую схему структуры из глобального кэша или строим рекурсивно один раз
	var schema *cachedSchema
	if val, ok := schemaCache.Load(structType); ok {
		schema = val.(*cachedSchema)
	} else {
		var builtMeta []FieldMeta
		l.extractMetaRecursive(structType, "", "", nil, &builtMeta)
		schema = &cachedSchema{meta: builtMeta}
		schemaCache.Store(structType, schema)
	}

	// 2. Арендуем срез экземпляров полей из пула памяти sync.Pool
	pSlice := instancePool.Get().(*[]FieldInstance)
	instances := *pSlice
	if cap(instances) < len(schema.meta) {
		instances = make([]FieldInstance, len(schema.meta))
	} else {
		instances = instances[:len(schema.meta)]
	}

	// Наполняем инстансы рантайм-ссылками полей текущей структуры по O(1) смещениям из кэша
	for i, meta := range schema.meta {
		instances[i] = FieldInstance{
			Meta:  meta,
			Value: structVal.FieldByIndex(meta.FieldIndex),
		}
	}

	// 3. Последовательное выполнение пайплайна провайдеров
	defaultProvider := &DefaultProvider{}
	if err := defaultProvider.Bind(instances); err != nil {
		instancePool.Put(pSlice)
		return err
	}

	// Флаги парсятся ровно один раз для ЭТОГО инстанса Loader (тесты изолированы)
	var flagErr error
	l.onceFlags.Do(func() {
		flagErr = l.flagProvider.Bind(instances)
	})
	if flagErr != nil {
		instancePool.Put(pSlice)
		return flagErr
	}

	envProvider := &EnvProvider{}
	if err := envProvider.Bind(instances); err != nil {
		instancePool.Put(pSlice)
		return err
	}

	// 4. Проверка обязательных полей (required)
	for _, inst := range instances {
		if inst.Meta.Required && l.isEmpty(inst.Value) {
			instancePool.Put(pSlice)
			return &ErrRequiredFieldMissing{FieldName: inst.Meta.FieldName}
		}
	}

	// Возвращаем переиспользованный срез обратно в пул
	*pSlice = instances
	instancePool.Put(pSlice)

	// 5. Вызов кастомной доменной валидации
	if validator, ok := cfg.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// extractMetaRecursive рекурсивно сканирует вложенные структуры и формирует плоский срез метаданных путей.
func (l *Loader) extractMetaRecursive(t reflect.Type, flagPrefix, envPrefix string, indexPrefix []int, metaList *[]FieldMeta) {
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)

		tagFlag := structField.Tag.Get(TagFlag)
		tagEnv := structField.Tag.Get(TagEnv)

		if structField.Type.Kind() == reflect.Struct {
			childFlag := l.join(flagPrefix, tagFlag, "-")
			if tagFlag == "" {
				childFlag = l.join(flagPrefix, strings.ToLower(structField.Name), "-")
			}

			childEnv := l.join(envPrefix, tagEnv, "_")
			if tagEnv == "" {
				childEnv = l.join(envPrefix, strings.ToUpper(structField.Name), "_")
			}

			childIndex := make([]int, len(indexPrefix), len(indexPrefix)+1)
			copy(childIndex, indexPrefix)
			childIndex = append(childIndex, i)

			l.extractMetaRecursive(structField.Type, childFlag, childEnv, childIndex, metaList)
			continue
		}

		if tagFlag == "" && tagEnv == "" {
			continue
		}

		// БЕЗОПАСНОЕ КОПИРОВАНИЕ СЛАЙСА: Формирует атомарный путь к конкретному примитивному поля
		currentIndex := make([]int, len(indexPrefix), len(indexPrefix)+1)
		copy(currentIndex, indexPrefix)
		currentIndex = append(currentIndex, i)

		*metaList = append(*metaList, FieldMeta{
			FieldName:  structField.Name,
			FlagName:   l.join(flagPrefix, tagFlag, "-"),
			EnvName:    l.envPrefix + l.join(envPrefix, tagEnv, "_"),
			Default:    structField.Tag.Get(TagDefault),
			Usage:      structField.Tag.Get(TagUsage),
			Required:   structField.Tag.Get(TagRequired) == "true",
			FieldIndex: currentIndex,
		})
	}
}

// join объединяет префиксы с тегами, используя указанный разделитель.
func (l *Loader) join(prefix, tag, sep string) string {
	if prefix == "" {
		return tag
	}
	if tag == "" {
		return prefix
	}
	return prefix + sep + tag
}

// isEmpty проверяет, инициализировано ли базовое рантайм-значение поля структуры.
func (l *Loader) isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint16:
		return v.Uint() == 0
	case reflect.Slice:
		return v.IsNil() || v.Len() == 0
	default:
		return false
	}
}
