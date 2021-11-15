package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/slack-go/slack"
)

type Bot struct {
	newsSource             NewsSource
	slackVerificationToken string
	slackClient            *slack.Client
}

// NewBot instantiates a new Bot
func NewBot(newsSource NewsSource, slackOAuthToken string, slackVerificationToken string) *Bot {
	return &Bot{
		newsSource:             newsSource,
		slackVerificationToken: slackVerificationToken,
		slackClient:            slack.New(slackOAuthToken),
	}
}

// HandleSlashCommand handles a slash command request
func (b *Bot) HandleSlashCommand(w http.ResponseWriter, r *http.Request) {
	s, err := slack.SlashCommandParse(r)
	if err != nil {
		log.Println("error parsing slash command:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// TODO: validate request with signing secret instead
	if !s.ValidateToken(b.slackVerificationToken) {
		log.Println("invalid token")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Would that ever happen?
	if s.Command != "/news" {
		log.Println("unexpected slash command:", s.Command)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// return 200 immediately to tell slack the payload was received.
	// command will be processed async
	w.WriteHeader(http.StatusOK)
	go b.processCommand(s.ChannelID, s.ResponseURL, strings.ToLower(s.Text))
}

func (b *Bot) processCommand(channelID string, responseURL string, params string) {
	// create new context not attached to the request, since this method is called async
	ctx := context.Background()
	switch {
	case strings.HasPrefix(params, "stories"):
		b.handleTopRequest(ctx, channelID, responseURL, params[7:])
		return
	default:
		b.handleHelpRequest(ctx, channelID, responseURL)
		return
	}
}

func (b *Bot) handleTopRequest(ctx context.Context, channelID string, responseURL string, params string) {
	params = strings.Trim(params, " ")
	if len(params) == 0 {
		// if no category is passed we default to top stories on the homepage
		params = "home"
	}

	// Fetch top 3 stories
	articles, err := b.newsSource.TopStories(ctx, params, 3)
	if err != nil {
		log.Println("error requesting top stories:", err)

		errMessage := "⚠️ Oops, something went wrong on our side. Try again later!"
		if err == ErrInvalidSection {
			errMessage = "⚠️ That's not a valid news section! Try requesting `/news help` to learn how to use this app!"
		}

		if _, _, err := b.slackClient.PostMessage(
			channelID,
			slack.MsgOptionResponseURL(responseURL, slack.ResponseTypeEphemeral),
			slack.MsgOptionText(errMessage, true),
		); err != nil {
			log.Println("error sending message:", err)
		}

		return
	}

	// build Block message and replace response
	var message slack.Blocks
	message.BlockSet = append(
		message.BlockSet,
		slack.NewHeaderBlock(&slack.TextBlockObject{
			Type: "plain_text",
			Text: "📢 Here are the top stories 📢",
		}),
	)

	for _, a := range articles {
		message.BlockSet = append(
			message.BlockSet,
			slack.NewSectionBlock(&slack.TextBlockObject{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*<%s|%s>*\n%s", a.URL, a.Title, a.Abstract),
			}, nil, nil),
			slack.NewContextBlock("", slack.TextBlockObject{
				Type: "plain_text",
				Text: a.PublishedAt,
			}),
			slack.NewDividerBlock(),
		)
	}

	if _, _, err := b.slackClient.PostMessage(channelID,
		slack.MsgOptionBlocks(message.BlockSet...),
		slack.MsgOptionResponseURL(responseURL, slack.ResponseTypeEphemeral),
	); err != nil {
		log.Println("error sending message:", err)
	}

}

// handleHelpRequest returns a Slack Block Kit structure that renders an interactive 'help' view
// every time an incorrect slash command is sent
func (b *Bot) handleHelpRequest(ctx context.Context, channelID string, responseURL string) {
	var message slack.Blocks
	message.BlockSet = append(message.BlockSet,
		slack.NewHeaderBlock(&slack.TextBlockObject{
			Type: "plain_text",
			Text: "See what's happening in the world 🗣",
		}),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: "mrkdwn",
				Text: "💡 Choose the news section you're interested in:",
			},
			nil,
			&slack.Accessory{
				SelectElement: &slack.SelectBlockElement{
					Type:    "static_select",
					Options: b.addNewsSectionsOptions(),
				},
			},
		),
	)

	if _, _, err := b.slackClient.PostMessage(channelID,
		slack.MsgOptionBlocks(message.BlockSet...),
		slack.MsgOptionResponseURL(responseURL, slack.ResponseTypeEphemeral),
	); err != nil {
		log.Println("error sending message:", err)
	}
}

// addNewsSectionsOptions loops through the available news sections a user can request
// and builds the appropriate options block object
func (b *Bot) addNewsSectionsOptions() []*slack.OptionBlockObject {
	var response []*slack.OptionBlockObject
	for _, section := range b.newsSource.SupportedSections() {
		response = append(response, &slack.OptionBlockObject{
			Text: &slack.TextBlockObject{
				Type: "plain_text",
				Text: b.newsSource.UserFriendlySection(section),
			},
			Value: section,
		})
	}
	return response
}

// HandleHelpInteraction handles a request coming from a 'help' view interaction
// It expects a slack interaction payload of type 'block_actions' containing the user's
// input (https://api.slack.com/reference/interaction-payloads/block-actions)
func (b *Bot) HandleHelpInteraction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Println("error parsing interactive request:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// NOTE: looks like this slack library has different implementation styles for interactive requests than
	// it does for slash commands (see handler above). I would probably revisit using this library again and
	// look for another more consistent one.
	//
	// We need to retrieve the 'payload' field and unmarshal in an InteractiveCallback object.
	payload := r.PostForm.Get("payload")
	var interaction slack.InteractionCallback
	if err := json.Unmarshal([]byte(payload), &interaction); err != nil {
		log.Println("error parsing interactive request:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// TODO: validate request with signing secret instead
	if interaction.Token != b.slackVerificationToken {
		log.Println("invalid token")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Check we have exactly one action coming in
	if len(interaction.ActionCallback.BlockActions) != 1 {
		log.Println("unexpected amount of actions received")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// return 200 immediately to tell slack the payload was received.
	// command will be processed async
	w.WriteHeader(http.StatusOK)

	action := interaction.ActionCallback.BlockActions[0]
	go b.handleTopRequest(context.Background(), interaction.Container.ChannelID, interaction.ResponseURL, action.SelectedOption.Value)
}
