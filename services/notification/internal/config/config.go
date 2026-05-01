package config

type Config struct {
	Env     string `env:"APP_ENV" envDefault:"local"`
	Port    string `env:"PORT" envDefault:"8081"`
	NatsURL string `env:"NATS_URL,required"`
}
