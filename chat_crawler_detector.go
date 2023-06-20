package main

import "strings"

type ChatCrawlerDetector struct {
	chatUserAgents []string
}

func NewChatCrawlerDetector() *ChatCrawlerDetector {
	return &ChatCrawlerDetector{
		chatUserAgents: []string{
			"slack",
			"google",
			"twitter",
			"facebook",
			"facebot",
			"whatsapp",
			"discord",
			"telegram",
			"skype",
			"linkedin",
			"viber",
		},
	}
}

func (c *ChatCrawlerDetector) IsChatCrawler(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	for _, chatUserAgent := range c.chatUserAgents {
		if strings.Contains(userAgent, chatUserAgent) {
			return true
		}
	}
	return false
}
