package confy

import (
	"reflect"
	"strings"
	"sync"
)

// Глобальный потокобезопасный кэш схемы типов. Предотвращает повторный разбор тегов и путей полей.
var schemaCache sync.Map

// Оптимизирующий пул объектов для повторного использования рантайм-срезов FieldInstance без аллокаций в куче.
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

// Load производит полную сборку конфигурации из трех слоев провайдеров с сохранением изолированности вызовов.
func (l *Loader) Load(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return &ErrInvalidConfigTarget{Kind: v.Kind()}
	}

	structVal := v.Elem()
	structType := reflect.TypeOf(cfg).Elem()

	// 1. Извлекаем схему структуры из глобального кэша или строим рекурсивно один раз
	var cachedMeta []FieldMeta
	if val, ok := schemaCache.Load(structType); ok {
		cachedMeta = val.([]FieldMeta)
	} else {
		var builtMeta []FieldMeta
		l.extractMetaRecursive(structType, "", "", nil, &builtMeta)
		schemaCache.Store(structType, builtMeta)
		cachedMeta = builtMeta
	}

	// 2. Арендуем срез экземпляров полей из пула sync.Pool
	pSlice := instancePool.Get().(*[]FieldInstance)
	instances := *pSlice
	if cap(instances) < len(cachedMeta) {
		instances = make([]FieldInstance, len(cachedMeta))
	} else {
		instances = instances[:len(cachedMeta)]
	}

	// Переносим рантайм-ссылки полей текущей структуры по O(1) смещениям из кэша
	for i, meta := range cachedMeta {
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

	// 4. Валидация обязательных полей (required)
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
			nextFlag := flagPrefix
			if tagFlag != "" {
				nextFlag = l.join(flagPrefix, tagFlag, "-")
			} else {
				nextFlag = l.join(flagPrefix, strings.ToLower(structField.Name), "-")
			}

			nextEnv := envPrefix
			if tagEnv != "" {
				nextEnv = l.join(envPrefix, tagEnv, "_")
			} else {
				nextEnv = l.join(envPrefix, strings.ToUpper(structField.Name), "_")
			}

			childIndex := make([]int, len(indexPrefix), len(indexPrefix)+1)
			copy(childIndex, indexPrefix)
			childIndex = append(childIndex, i)

			l.extractMetaRecursive(structField.Type, nextFlag, nextEnv, childIndex, metaList)
			continue
		}

		if tagFlag == "" && tagEnv == "" {
			continue
		}

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

func (l *Loader) join(prefix, tag, sep string) string {
	if prefix == "" {
		return tag
	}
	if tag == "" {
		return prefix
	}
	return prefix + sep + tag
}

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
