package config

import "time"

type Config struct {
	Env         string        `env:"APP_ENV" envDefault:"local"`
	Port        string        `env:"PORT" envDefault:"8083"`
	PostgresURL string        `env:"POSTGRES_URL,required"`
	NatsURL     string        `env:"NATS_URL,required"`
	PSPBaseURL  string        `env:"PSP_BASE_URL"`
	PSPTimeout  time.Duration `env:"PSP_TIMEOUT"`
}
