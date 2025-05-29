package config

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type Config struct {
	Env         string
	Version string
	HTTPServer  HTTPServer
}

type HTTPServer struct {
	Address     string
	Timeout     time.Duration
	User        string
	Password    string
}

type jsonConfig struct {
	Env string `json:"env"`
	Version string `json:"version"`
	HTTPServer jsonHTTPServer `json:"http_server"`
}

type jsonHTTPServer struct {
	Address string `json:"address"`
	Timeout string `json:"timeout"`
}

var (
	defaultAddress = "localhost:8080"
	defaulTimeout = 4 * time.Second
	defaultEnv = "local"
	defaultVersion = "0.0.0"
)

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH переменная окружения не установлена")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Файл конфигурации не существует: %s", configPath)
	}

	cfg := Config{
		Env: defaultEnv,
		Version: defaultVersion,
		HTTPServer: HTTPServer{
			Address: defaultAddress,
			Timeout: defaulTimeout,
		},
	}

	fileBytes, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Ошибка чтения файла конфигурации %s: %v", configPath, err)
	}

	var jsonCfg jsonConfig
	if err := json.Unmarshal(fileBytes, &jsonCfg); err != nil {
		log.Fatalf("Ошибка разбора JSON из %s: %v", configPath, err)
	}

	if jsonCfg.Env != "" {
		cfg.Env = jsonCfg.Env
	}

	if jsonCfg.Version != "" {
		cfg.Version = jsonCfg.Version
	}

	if jsonCfg.HTTPServer.Address != "" {
		cfg.HTTPServer.Address = jsonCfg.HTTPServer.Address
	}

	if jsonCfg.HTTPServer.Timeout != "" {
		parsedDur, err := time.ParseDuration(jsonCfg.HTTPServer.Timeout)
		if err != nil {
			log.Fatalf("Ошибка парсинга http_server.timeout из JSON ('%s'): %v", jsonCfg.HTTPServer.Timeout, err)
		}
		cfg.HTTPServer.Timeout = parsedDur
	}

	if envVal := os.Getenv("ENV"); envVal != "" {
		cfg.Env = envVal
	}

	if envVal := os.Getenv("VERSION"); envVal != "" {
		cfg.HTTPServer.Address = envVal
	}

	if envVal := os.Getenv("HTTP_SERVER_ADDRESS"); envVal != "" {
		cfg.HTTPServer.Address = envVal
	}

	if envValStr := os.Getenv("HTTP_SERVER_TIMEOUT"); envValStr != "" {
		parsedDur, err := time.ParseDuration(envValStr)
		if err != nil {
			log.Fatalf("Неверный формат для переменной окружения HTTP_SERVER_TIMEOUT ('%s'): %v", envValStr, err)
		}
		cfg.HTTPServer.Timeout = parsedDur
	}

	return &cfg
}