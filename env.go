package main

import (
	"os"
	"strconv"
)

func GetEnvFileOrPanic(env string) string {
	switch env {
	case "prod":
		return ".env"
	case "dev", "":
		return ".env.dev"
	}
	panic("invalid env")
}

func IsDev() bool {
	return os.Getenv("ENV") == "dev" || os.Getenv("ENV") == ""
}

func GetRedisHostOrPanic() string {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		panic("REDIS_HOST is not set")
	}
	return host
}

func GetRedisPortOrPanic() string {
	port := os.Getenv("REDIS_PORT")
	if port == "" {
		panic("REDIS_PORT is not set")
	}
	return port
}

func GetRedisPassword() string {
	password := os.Getenv("REDIS_PASSWORD")
	return password
}

func GetRedisDBOrPanic() int {
	db := os.Getenv("REDIS_DB")
	if db == "" {
		panic("REDIS_DB is not set")
	}
	dbInt, err := strconv.Atoi(db)
	if err != nil {
		panic("REDIS_DB is not a valid integer")
	}
	return dbInt
}

func GetPublicKeyOrPanic() string {
	key := os.Getenv("PUBLIC_KEY")
	if key == "" {
		panic("PUBLIC_KEY is not set")
	}
	return key
}

func GetHostOrPanic() string {
	host := os.Getenv("HOST")
	if host == "" {
		panic("HOST is not set")
	}
	return host
}

func GetHTTPPortOrPanic() string {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		panic("HTTP_PORT is not set")
	}
	return port
}

func GetSSHPortOrPanic() string {
	port := os.Getenv("SSH_PORT")
	if port == "" {
		panic("SSH_PORT is not set")
	}
	return port
}
