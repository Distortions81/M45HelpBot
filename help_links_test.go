package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

var helpURLPattern = regexp.MustCompile(`https?://[^\s<>"\]]+`)

type helpURL struct {
	raw     string
	parsed  *url.URL
	context string
}

type discordHelpLink struct {
	guildID   string
	channelID string
	messageID string
}

func TestHelpReplyLinksAreWellFormed(t *testing.T) {
	links := loadHelpURLs(t)
	if len(links) == 0 {
		t.Fatal("helps.json does not contain any reply links")
	}

	for _, link := range links {
		if link.parsed.Scheme != "http" && link.parsed.Scheme != "https" {
			t.Errorf("%s: unsupported URL scheme in %q", link.context, link.raw)
		}
		if link.parsed.Host == "" {
			t.Errorf("%s: missing URL host in %q", link.context, link.raw)
		}

		if isDiscordURL(link.parsed) {
			if _, err := parseDiscordHelpLink(link.parsed); err != nil {
				t.Errorf("%s: malformed Discord link %q: %v", link.context, link.raw, err)
			}
		}
	}
}

func TestHelpExternalLinksReachable(t *testing.T) {
	if os.Getenv("LINKCHECK") != "1" {
		t.Skip("set LINKCHECK=1 to run live link checks")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	checked := 0

	for _, link := range loadHelpURLs(t) {
		if isDiscordURL(link.parsed) {
			continue
		}
		checked++

		if err := checkHTTPLink(client, link.parsed.String()); err != nil {
			t.Errorf("%s: %q is not reachable: %v", link.context, link.raw, err)
		}
	}

	if checked == 0 {
		t.Skip("no non-Discord links found in helps.json")
	}
}

func TestHelpDiscordLinksResolve(t *testing.T) {
	if os.Getenv("LINKCHECK") != "1" {
		t.Skip("set LINKCHECK=1 to run live link checks")
	}

	token := os.Getenv("LINKCHECK_DISCORD_TOKEN")
	if token == "" {
		token = os.Getenv("DISCORD_TOKEN")
	}
	if token == "" {
		t.Skip("set LINKCHECK_DISCORD_TOKEN or DISCORD_TOKEN to verify Discord channel/message links")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	checked := 0

	for _, link := range loadHelpURLs(t) {
		if !isDiscordURL(link.parsed) {
			continue
		}
		checked++

		discordLink, err := parseDiscordHelpLink(link.parsed)
		if err != nil {
			t.Errorf("%s: malformed Discord link %q: %v", link.context, link.raw, err)
			continue
		}

		if err := checkDiscordHelpLink(client, token, discordLink); err != nil {
			t.Errorf("%s: %q does not resolve through the Discord API: %v", link.context, link.raw, err)
		}
	}

	if checked == 0 {
		t.Skip("no Discord links found in helps.json")
	}
}

func loadHelpURLs(t *testing.T) []helpURL {
	t.Helper()

	file, err := os.ReadFile("helps.json")
	if err != nil {
		t.Fatalf("read helps.json: %v", err)
	}

	var helps []HelpsListData
	if err := json.Unmarshal(file, &helps); err != nil {
		t.Fatalf("parse helps.json: %v", err)
	}

	var links []helpURL
	for _, helpType := range helps {
		for helpIndex, help := range helpType.Data {
			for lineIndex, line := range help.ReplyLines {
				matches := helpURLPattern.FindAllString(line, -1)
				for _, match := range matches {
					raw := strings.TrimRight(match, ".,;:!?)")
					parsed, err := url.Parse(raw)
					if err != nil {
						t.Fatalf("%s[%d].ReplyLines[%d]: parse %q: %v", helpType.Name, helpIndex, lineIndex, raw, err)
					}
					links = append(links, helpURL{
						raw:     raw,
						parsed:  parsed,
						context: fmt.Sprintf("%s[%d].ReplyLines[%d]", helpType.Name, helpIndex, lineIndex),
					})
				}
			}
		}
	}

	return links
}

func checkHTTPLink(client *http.Client, rawURL string) error {
	statusCode, body, err := requestURL(client, http.MethodHead, rawURL, "")
	if err == nil && statusCode < http.StatusBadRequest {
		return nil
	}

	statusCode, body, getErr := requestURL(client, http.MethodGet, rawURL, "")
	if getErr != nil {
		if err != nil {
			return fmt.Errorf("HEAD failed with %v; GET failed with %v", err, getErr)
		}
		return getErr
	}

	if statusCode >= http.StatusBadRequest {
		return fmt.Errorf("HTTP %d: %s", statusCode, body)
	}
	return nil
}

func checkDiscordHelpLink(client *http.Client, token string, link discordHelpLink) error {
	channelURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s", link.channelID)
	statusCode, body, err := requestURL(client, http.MethodGet, channelURL, discordAuthHeader(token))
	if err != nil {
		return fmt.Errorf("channel lookup failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("channel lookup returned HTTP %d: %s", statusCode, body)
	}
	if err := checkDiscordChannelGuild(body, link.guildID); err != nil {
		return err
	}

	if link.messageID == "" {
		return nil
	}

	messageURL := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages/%s", link.channelID, link.messageID)
	statusCode, body, err = requestURL(client, http.MethodGet, messageURL, discordAuthHeader(token))
	if err != nil {
		return fmt.Errorf("message lookup failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("message lookup returned HTTP %d: %s", statusCode, body)
	}

	return nil
}

func requestURL(client *http.Client, method, rawURL, authorization string) (int, string, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "goDiscInfoBot-linkcheck")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return resp.StatusCode, "", err
	}

	return resp.StatusCode, strings.TrimSpace(string(body)), nil
}

func checkDiscordChannelGuild(body, expectedGuildID string) error {
	var channel struct {
		GuildID string `json:"guild_id"`
	}
	if err := json.Unmarshal([]byte(body), &channel); err != nil {
		return fmt.Errorf("channel response was not valid JSON: %w", err)
	}
	if channel.GuildID != "" && channel.GuildID != expectedGuildID {
		return fmt.Errorf("channel belongs to guild %s, link uses guild %s", channel.GuildID, expectedGuildID)
	}
	return nil
}

func isDiscordURL(parsed *url.URL) bool {
	host := strings.ToLower(parsed.Hostname())
	return host == "discord.com" || host == "www.discord.com"
}

func parseDiscordHelpLink(parsed *url.URL) (discordHelpLink, error) {
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 && len(parts) != 4 {
		return discordHelpLink{}, fmt.Errorf("expected /channels/{guildID}/{channelID} or /channels/{guildID}/{channelID}/{messageID}")
	}
	if parts[0] != "channels" {
		return discordHelpLink{}, fmt.Errorf("expected path to start with /channels")
	}
	for _, id := range parts[1:] {
		if !isDiscordSnowflake(id) {
			return discordHelpLink{}, fmt.Errorf("invalid Discord snowflake %q", id)
		}
	}

	link := discordHelpLink{
		guildID:   parts[1],
		channelID: parts[2],
	}
	if len(parts) == 4 {
		link.messageID = parts[3]
	}
	return link, nil
}

func isDiscordSnowflake(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func discordAuthHeader(token string) string {
	if strings.HasPrefix(token, "Bot ") || strings.HasPrefix(token, "Bearer ") {
		return token
	}
	return "Bot " + token
}
