package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	cfg := initConfig()

	nytClient := NewNYTimes(cfg.nytAPIKey)

	bot := NewBot(nytClient, cfg.slackBotToken, cfg.slackVerificationToken)

	r := http.NewServeMux()
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello world!")
	})
	r.HandleFunc("/receive", bot.HandleSlashCommand)
	r.HandleFunc("/receive/help", bot.HandleHelpInteraction)

	serverAddress := fmt.Sprintf("0.0.0.0:%s", "80")
	server := &http.Server{Addr: serverAddress, Handler: r}

	fmt.Printf("Serving at http://%s/", serverAddress)

	// start service in a go routine to support graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("error shutting down bot service")
		}
	}()

	// wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	fmt.Printf("caught signal %s, shutting down...", sig)

	// The context is used to inform the server it has X seconds to finish the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Println("error shutting down server cleanly", err)
	} else {
		fmt.Println("gracefully shut down bot service")
	}
}

// ----//----

type Config struct {
	nytAPIKey              string
	slackBotToken          string
	slackVerificationToken string
}

func initConfig() Config {
	if os.Getenv("ENV") == "taina-local" {
		godotenv.Load()
	}
	return Config{
		nytAPIKey:              os.Getenv("NYT_API_KEY"),
		slackBotToken:          os.Getenv("SLACK_BOT_TOKEN"),
		slackVerificationToken: os.Getenv("SLACK_VERIFICATION_TOKEN"),
	}
}
