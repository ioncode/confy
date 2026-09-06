package confy_test

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ioncode/confy"
)

type TestDB struct {
	Host string `flag:"host" env:"HOST" default:"localhost"`
	Port uint16 `flag:"port" env:"PORT" default:"5432"`
}

type TestConfig struct {
	Env     string        `flag:"env" env:"ENV" default:"local"`
	Allowed []string      `flag:"allowed" env:"ALLOWED" default:"localhost,127.0.0.1"`
	Timeout time.Duration `flag:"timeout" env:"TIMEOUT" default:"5s"`
	DB      TestDB
}

type FailConfig struct {
	Secret string `flag:"secret" env:"SECRET" required:"true"`
}

func TestLoader_CustomErrors(t *testing.T) {
	t.Parallel()

	loader := confy.New("TEST_", []string{})
	var cfg FailConfig

	err := loader.Load(&cfg)
	var reqErr *confy.ErrRequiredFieldMissing

	if err == nil {
		t.Fatal("Ожидали ошибку для незаполненного поля, получили nil")
	}

	if !errors.As(err, &reqErr) {
		t.Fatalf("Ожидали ошибку типа ErrRequiredFieldMissing, получили: %v", err)
	}
}

func TestLoader_NestedAndCached(t *testing.T) {
	t.Parallel()

	args := []string{"-env", "prod", "-db-host", "10.0.0.1", "-db-port", "9000"}
	loader := confy.New("TEST_", args)
	var cfg TestConfig

	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Ошибка загрузки: %v", err)
	}

	if cfg.Env != "prod" || cfg.DB.Host != "10.0.0.1" || cfg.DB.Port != 9000 {
		t.Errorf("Данные применились некорректно: %+v", cfg)
	}
}

func TestLoader_EnvPriority(t *testing.T) {
	defer os.Clearenv()

	if err := os.Setenv("TEST_ENV", "staging"); err != nil {
		t.Fatalf("Не удалось установить переменную окружения TEST_ENV: %v", err)
	}
	if err := os.Setenv("TEST_DB_PORT", "1111"); err != nil {
		t.Fatalf("Не удалось установить переменную окружения TEST_DB_PORT: %v", err)
	}

	args := []string{"-env", "production", "-db-port", "2222"}
	loader := confy.New("TEST_", args)
	var cfg TestConfig

	if err := loader.Load(&cfg); err != nil {
		t.Fatalf("Ошибка загрузки: %v", err)
	}

	if cfg.Env != "staging" || cfg.DB.Port != 1111 {
		t.Errorf("ENV должен был перетереть флаги запуска: %+v", cfg)
	}
}

func BenchmarkLoader_LoadWithCache(b *testing.B) {
	loader := confy.New("BENCH_", []string{"-env", "benchmark"})
	var cfg TestConfig

	_ = loader.Load(&cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = loader.Load(&cfg)
	}
}
