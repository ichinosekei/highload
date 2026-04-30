package config

type Config struct {
	Env           string `env:"APP_ENV"`
	Port          string `env:"PORT,required"`
	PostgresURL   string `env:"POSTGRES_URL,required"`
	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPassword string `env:"REDIS_PASSWORD,required"`
	NatsURL       string `env:"NATS_URL,required"`
}
