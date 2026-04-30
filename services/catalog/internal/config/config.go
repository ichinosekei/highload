package config

import "time"

type Config struct {
	Env            string        `env:"APP_ENV"`
	Port           string        `env:"PORT,required"`
	PostgresURL    string        `env:"POSTGRES_URL,required"`
	RedisAddr      string        `env:"REDIS_ADDR,required"`
	RedisPassword  string        `env:"REDIS_PASSWORD,required"`
	MeiliHost      string        `env:"MEILI_HOST,required"`
	MeiliKey       string        `env:"MEILI_MASTER_KEY,required"`
	CacheTTLSearch time.Duration `env:"CACHE_TTL_SEARCH"`
	CacheTTLMenu   time.Duration `env:"CACHE_TTL_MENU"`
}
