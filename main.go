package main

import (
	"fmt"
	"github.com/gliderlabs/ssh"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"log"
	"os"
	"time"
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

	chatCrawlerDetector := NewChatCrawlerDetector()
	tunnelManager := NewTunnelManager()
	httpServer := NewHTTPServer(sugar, redisStore, tunnelManager, chatCrawlerDetector)

	go func() {
		addr := fmt.Sprintf(":%s", GetHTTPPortOrPanic())
		sugar.Infow("starting http server", "addr", addr)
		if err := httpServer.ListenAndServe(addr); err != nil {
			sugar.Fatal("failed to start http server", err)
		}
	}()

	go func() {
		for range time.Tick(3 * time.Minute) {
			tunnelDelCount := tunnelManager.CleanUp()
			sugar.Infow("cleaned up tunnels", "deletedTunnels", tunnelDelCount)
		}
	}()

	privateKey := ssh.HostKeyFile(GetPublicKeyOrPanic())
	sshServer := NewSSHServer(sugar, redisStore, tunnelManager, GetHostOrPanic())
	sugar.Infow("starting ssh server", "addr", fmt.Sprintf(":%s", GetSSHPortOrPanic()))
	if err := sshServer.ListenAndServe(fmt.Sprintf(":%s", GetSSHPortOrPanic()), nil, privateKey); err != nil {
		sugar.Errorw("failed to start ssh server", "err", err)
	}
}
