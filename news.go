package main

import (
	"context"
	"errors"

	"github.com/tainacleal/nyt-go/nyttop"
)

var (
	ErrInvalidSection = errors.New("invalid section")
)

// Article holds the information we need to render a Slack Block response
type Article struct {
	Title    string
	Abstract string
	URL      string
}

// NewsSource is an interface that should be implemented by types that can retrieve top news stories
type NewsSource interface {
	TopStories(ctx context.Context, section string, topN int) ([]Article, error)
}

// ----//----

// NYTimes can communicate with The New York Times API. It implements the NewsSource interface.
type NYTimes struct {
	APIKey string
}

func NewNYTimes(apiKey string) *NYTimes {
	return &NYTimes{
		APIKey: apiKey,
	}
}

// TopStories retrieves the top stories from The NY Times.
func (nyt *NYTimes) TopStories(ctx context.Context, section string, topN int) ([]Article, error) {
	client := nyttop.New(nyt.APIKey)
	articles, err := client.TopNStories(ctx, nyttop.Section(section), topN)
	if err != nil {
		if err == nyttop.ErrInvalidSection {
			return nil, ErrInvalidSection
		}
		return nil, err
	}

	result := []Article{}
	for _, a := range articles {
		result = append(result, Article{
			Title:    a.Title,
			Abstract: a.Abstract,
			URL:      a.ShortURL,
		})
	}

	return result, nil
}
