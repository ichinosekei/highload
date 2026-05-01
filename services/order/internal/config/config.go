package config

type Config struct {
	Env           string `env:"APP_ENV" envDefault:"local"`
	Port          string `env:"PORT" envDefault:"8082"`
	PostgresURL   string `env:"POSTGRES_URL,required"`
	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPassword string `env:"REDIS_PASSWORD,required"`
	NatsURL       string `env:"NATS_URL,required"`
}
