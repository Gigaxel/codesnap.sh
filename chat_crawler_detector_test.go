package main

import "testing"

func TestChatCrawlerDetector_IsChatCrawler(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      bool
	}{
		{
			name:      "slack",
			userAgent: "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
			want:      true,
		},
		{
			name:      "google",
			userAgent: "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			want:      true,
		},
		{
			name:      "twitter",
			userAgent: "Twitterbot/1.0",
			want:      true,
		},
		{
			name:      "iMessage",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/601.2.4 (KHTML, like Gecko) Version/9.0.1 Safari/601.2.4 facebookexternalhit/1.1 Facebot Twitterbot/1.0",
			want:      true,
		},
		{
			name:      "normal browser request",
			userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/601.2.4 (KHTML, like Gecko) Version/9.0.1 Safari/601.2.4",
			want:      false,
		},
	}

	c := NewChatCrawlerDetector()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.IsChatCrawler(tt.userAgent); got != tt.want {
				t.Errorf("IsChatCrawler() = %v, want %v", got, tt.want)
			}
		})
	}
}
