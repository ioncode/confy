# Confy 🚀

Легковесная, высокопроизводительная библиотека для декларативной конфигурации Go-приложений на базе принципов **чистой архитектуры (Clean Architecture)**. Она автоматически разбирает структурные теги, генерирует авто-справку в CLI и строго соблюдает концепцию приоритетов **12-Factor App**.

## ✨ Особенности

- **Zero Dependencies:** Никаких сторонних библиотек, только стандартные пакеты `reflect` и `flag`.
- **Чистая Архитектура:** Логика парсинга полностью отделена от бизнес-логики. Любые новые источники (YAML, Vault, Consul) подключаются созданием одного инфраструктурного провайдера.
- **Строгий приоритет:** Переменные окружения > Флаги CLI > Тег `default` > Текущее состояние полей.
- **Высокая производительность:** Использование глобального кэша схем структур (`sync.Map`) исключает повторную рефлексию, а пул объектов (`sync.Pool`) минимизирует нагрузку на Garbage Collector в рантайме.
- **Умная вложенность:** Автоматически строит каскад префиксов для вложенных структур без багов перезаписи памяти (поле `DB.Port` -> флаг `-db-host`, переменная `APP_DB_HOST`).
- **Поддержка сложных типов:** Нативно обрабатывает `time.Duration` (строки вида `"5s"`, `"1h"`) и списки `[]string` через запятую.
- **Fail-Fast валидация:** Поддержка тега `required:"true"` с возвратом строго типизированных ошибок доменного уровня.

## 📦 Установка

```bash
go get github.com/ioncode/confy
```

## 🛠️ Быстрый старт

```go
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"://github.com"
)

type DBConfig struct {
	Host string `flag:"host" env:"HOST" default:"localhost" usage:"Хост базы данных"`
	Port uint16 `flag:"port" env:"PORT" default:"5432" usage:"Порт базы данных"`
}

type Config struct {
	Env     string        `flag:"env" env:"ENV" default:"local" usage:"Окружение (local, dev, prod)"`
	Timeout time.Duration `flag:"timeout" env:"TIMEOUT" default:"5s" usage:"Таймаут операций"`
	DB      DBConfig      // Вложенная структура без явных тегов
}

func main() {
	var cfg Config

	// "APP_" - префикс для переменных окружения
	loader := confy.New("APP_", os.Args[1:])
	
	if err := loader.Load(&cfg); err != nil {
		log.Fatalf("Ошибка инициализации: %v", err)
	}

	fmt.Printf("Запущено. Env: %s, DB Host: %s, Timeout: %v\n", cfg.Env, cfg.DB.Host, cfg.Timeout)
}
```

## 📖 Справка запуска (--help)

Запустите приложение с флагом `--help` или `-help`, чтобы увидеть автоматически сгенерированную и выровненную таблицу параметров:

```text
Использование приложения:

  ФЛАГ       ПЕРЕМЕННАЯ ОКРУЖЕНИЯ  ЗНАЧЕНИЕ ПО УМОЛЧАНИЮ  ОПИСАНИЕ
  ----       --------------------  ---------------------  ---------
  -env       APP_ENV               local                  Окружение (local, dev, prod)
  -timeout   APP_TIMEOUT           5s                     Таймаут операций
  -db-host   APP_DB_HOST           localhost              Хост базы данных
  -db-port   APP_DB_PORT           5432                   Порт базы данных

Переменные окружения имеют приоритет над флагами командной строки.
```

## 📜 Лицензия (MIT License)

Copyright (c) 2026 Andrey Ponteleev

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
