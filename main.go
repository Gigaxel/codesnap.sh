package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gliderlabs/ssh"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	env := os.Getenv("ENV")
	err := godotenv.Load(GetEnvFileOrPanic(env))
	if err != nil {
		log.Fatal("error loading .env file", err)
		return
	}

	var logger *zap.Logger
	switch {
	case IsDev():
		logger, err = zap.NewDevelopment()
		if err != nil {
			log.Fatal("failed to initialize dev zap logger", err)
			return
		}
	default:
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatal("failed to initialize prod zap logger", err)
			return
		}
	}

	logger.Debug("running in debug mode")

	defer logger.Sync()
	sugar := logger.Sugar()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", GetRedisHostOrPanic(), GetRedisPortOrPanic()),
		Password: GetRedisPassword(),
		DB:       GetRedisDBOrPanic(),
	})
	redisStore := NewRedisStore(redisClient)
	httpServer := NewHTTPServer(sugar, redisStore)

	go func() {
		addr := fmt.Sprintf(":%s", GetHTTPPortOrPanic())
		sugar.Infow("starting http server", "addr", addr)
		if err := httpServer.ListenAndServe(addr); err != nil {
			sugar.Fatal("failed to start http server", err)
		}
	}()

	privateKey := ssh.HostKeyFile(GetPublicKeyOrPanic())
	sshServer := NewSSHServer(sugar, redisStore, GetHostOrPanic())

	sugar.Infow("starting ssh server", "addr", fmt.Sprintf(":%s", GetSSHPortOrPanic()))
	if err := sshServer.ListenAndServe(fmt.Sprintf(":%s", GetSSHPortOrPanic()), nil, privateKey); err != nil {
		// throws an error
		sugar.Errorw("failed to start ssh server", "err", err)
	}
}
