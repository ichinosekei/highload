package config

type Config struct {
	Env     string `env:"APP_ENV"`
	Port    string `env:"PORT"`
	NatsURL string `env:"NATS_URL,required"`
}
