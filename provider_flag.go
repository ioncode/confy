package confy

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"text/tabwriter"
	"time"
)

// FlagProvider отвечает за разбор, регистрацию и маппинг аргументов командной строки (CLI флагов).
type FlagProvider struct {
	fs   *flag.FlagSet
	args []string
}

// NewFlagProvider инициализирует новый изолированный FlagProvider с собственным набором флагов.
func NewFlagProvider(args []string) *FlagProvider {
	return &FlagProvider{
		fs:   flag.NewFlagSet("confy", flag.ContinueOnError),
		args: args,
	}
}

// InitAndBind динамически регистрирует флаги на базе метаданных и производит их разбор (парсинг).
func (p *FlagProvider) InitAndBind(fields []FieldInstance) error {
	var defers []func()

	for _, f := range fields {
		if f.Meta.FlagName == "" {
			continue
		}

		switch f.Value.Kind() {
		case reflect.String:
			p.fs.StringVar(f.Value.Addr().Interface().(*string), f.Meta.FlagName, f.Value.String(), f.Meta.Usage)
		case reflect.Uint16:
			var temp uint
			p.fs.UintVar(&temp, f.Meta.FlagName, uint(f.Value.Uint()), f.Meta.Usage)
			defers = append(defers, func(v reflect.Value, t *uint) func() {
				return func() { v.SetUint(uint64(*t)) }
			}(f.Value, &temp))
		case reflect.Int, reflect.Int64:
			if f.Value.Type() == reflect.TypeOf(time.Duration(0)) {
				var tempStr string
				p.fs.StringVar(&tempStr, f.Meta.FlagName, f.Value.Interface().(time.Duration).String(), f.Meta.Usage)
				defers = append(defers, func(v reflect.Value, t *string) func() {
					return func() {
						if *t != "" {
							if d, err := time.ParseDuration(*t); err == nil {
								v.Set(reflect.ValueOf(d))
							}
						}
					}
				}(f.Value, &tempStr))
			} else {
				p.fs.IntVar(f.Value.Addr().Interface().(*int), f.Meta.FlagName, int(f.Value.Int()), f.Meta.Usage)
			}
		case reflect.Bool:
			p.fs.BoolVar(f.Value.Addr().Interface().(*bool), f.Meta.FlagName, f.Value.Bool(), f.Meta.Usage)
		case reflect.Slice:
			if f.Value.Type().Elem().Kind() == reflect.String {
				p.fs.Var(&stringSliceValue{value: f.Value.Addr().Interface().(*[]string)}, f.Meta.FlagName, f.Meta.Usage)
			}
		}
	}

	p.fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Использование приложения:\n\n")
		w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  ФЛАГ\tПЕРЕМЕННАЯ ОКРУЖЕНИЯ\tЗНАЧЕНИЕ ПО УМОЛЧАНИЮ\tОПИСАНИЕ")
		_, _ = fmt.Fprintln(w, "  ----\t--------------------\t---------------------\t---------")
		for _, f := range fields {
			envStr := f.Meta.EnvName
			if envStr == "" {
				envStr = "-"
			}
			_, _ = fmt.Fprintf(w, "  -%s\t%s\t%v\t%s\n", f.Meta.FlagName, envStr, f.Value.Interface(), f.Meta.Usage)
		}
		_ = w.Flush()
		_, _ = fmt.Fprintf(os.Stderr, "\nПеременные окружения имеют приоритет над флагами командной строки.\n")
	}

	if err := p.fs.Parse(p.args); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		return err
	}

	for _, fn := range defers {
		fn()
	}
	return nil
}

// Bind проверяет, были ли флаги уже распарсены для текущего жизненного цикла, исключая повторные аллокации флагов.
func (p *FlagProvider) Bind(fields []FieldInstance) error {
	if p.fs.Parsed() {
		return nil
	}
	return p.InitAndBind(fields)
}
