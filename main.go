package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	cfg := initConfig()

	nytClient := NewNYTimes(cfg.nytAPIKey)

	bot := NewBot(nytClient, cfg.slackBotToken, cfg.slackVerificationToken)

	r := http.NewServeMux()
	r.HandleFunc("/receive", bot.HandleSlashCommand)
	r.HandleFunc("/receive/help", bot.HandleHelpInteraction)

	serverAddress := fmt.Sprintf("0.0.0.0:%s", "80")
	server := &http.Server{Addr: serverAddress, Handler: r}

	fmt.Printf("Serving at http://%s/", serverAddress)
	server.ListenAndServe()
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
