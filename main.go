package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"M45HelpBot/cwlog"
	"M45HelpBot/sclean"
	"github.com/bwmarrin/discordgo"
)

var (
	skipThrottle bool

	helpsFile string
)

func main() {
	token := flag.String("token", "", "discord token")
	role := flag.String("staffid", "", "discord role ID for moderator/staff")
	staffChan := flag.String("staffChannel", "", "specify a staff-only channel")
	guildid := flag.String("guildid", "", "discord guild id")
	testMode := flag.Bool("testmode", false, "skip throttle check")
	helpPath := flag.String("helpFilePath", "helps.json", "Specify path to helps file.")
	flag.Parse()

	discToken = *token
	staffRole = *role
	guildID = *guildid
	skipThrottle = *testMode
	helpsFile = *helpPath
	staffChannel = *staffChan

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, rebootTime)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cwlog.StartCWLog()
	defer cwlog.CloseCWLog()
	cwlog.DoLog("Starting goDiscInfoBot.")
	if strings.TrimSpace(guildID) == "" {
		return fmt.Errorf("Discord guild ID not set")
	}
	if !readHelps() {
		return fmt.Errorf("could not load help file %q", helpsFile)
	}
	if ctx.Err() != nil {
		return nil
	}
	type connectionResult struct {
		bot *discordgo.Session
		err error
	}
	connected := make(chan connectionResult)
	go func(token string) {
		bot, err := startbot(ctx, token)
		select {
		case connected <- connectionResult{bot, err}:
		case <-ctx.Done():
			if bot != nil {
				bot.Close()
			}
		}
	}(discToken)
	// Open can block in the gateway handshake. Signals must still stop the bot.
	select {
	case result := <-connected:
		if result.err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return result.err
		}
		defer result.bot.Close()
	case <-ctx.Done():
		return nil
	}
	<-ctx.Done()
	return nil
}

func startbot(ctx context.Context, token string) (*discordgo.Session, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("Discord token not set")
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt*5) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cwlog.DoLog("Starting Discord bot...")
		bot, err := newBotSession(token)
		if err == nil {
			err = bot.Open()
			if err == nil {
				return bot, nil
			}
			bot.Close()
		}
		lastErr = err
		cwlog.DoLog(fmt.Sprintf("Discord connection attempt %d failed: %v", attempt+1, err))
	}
	return nil, fmt.Errorf("Discord connection failed after %d attempts: %w", maxAttempts, lastErr)
}

func newBotSession(token string) (*discordgo.Session, error) {
	bot, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	bot.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentMessageContent
	bot.LogLevel = discordgo.LogWarning
	// Register once per session: READY can be delivered again after reconnecting.
	bot.AddHandler(BotReady)
	bot.AddHandler(MessageCreate)
	return bot, nil
}

func BotReady(s *discordgo.Session, r *discordgo.Ready) {

	/* Set the bot's Discord status message */
	botstatus := "m45sci.xyz"
	errc := s.UpdateGameStatus(0, botstatus)
	if errc != nil {
		cwlog.DoLog(errc.Error())
	}

	cwlog.DoLog("Discord bot ready.")
}

func MessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	if s == nil || m == nil || m.Message == nil || m.Author == nil ||
		m.Author.Bot || m.WebhookID != "" || m.GuildID == "" || m.GuildID != guildID {
		return
	}

	if s.State != nil {
		s.State.RLock()
		isSelf := s.State.User != nil && m.Author.ID == s.State.User.ID
		s.State.RUnlock()
		if isSelf {
			return
		}
	}

	filterMessages(s, m)
}

func filterMessages(s *discordgo.Session, m *discordgo.MessageCreate) {
	staffMode := false

	//Switch lists if user is staff
	searchList := HelpsListData{}

	if staffRole != "" && m.Member != nil {
		for _, role := range m.Member.Roles {
			if role == staffRole {
				staffMode = true
				if staffChannel != "" &&
					m.ChannelID != staffChannel {
					return
				}
				break
			}
		}
	}

	for _, help := range helpsList {
		if !staffMode && strings.EqualFold(help.Name, "main") {
			searchList = help
			break
		} else if staffMode && strings.EqualFold(help.Name, "staff") {
			searchList = help
			break
		}
	}

	if len(searchList.Data) == 0 {
		cwlog.DoLog("No helps data found.")
		return
	}

	outLines := helpReplyLines(m.Content, searchList.Data)
	if len(outLines) > 0 {
		sendHelpReply(s, m, strings.Join(outLines, "\n"))
	}
}

func helpReplyLines(content string, helps []helpData) []string {
	msgLower := strings.ToLower(sclean.RemoveDiscordMarkdown(content))
	msgLower = sclean.StripControlAndSubSpecial(msgLower)
	msgLower = strings.Join(strings.Fields(msgLower), " ")
	msgWild := sclean.AlphaNumOnly(strings.ReplaceAll(msgLower, " the ", " "))
	msgWords := strings.Fields(msgLower)
	for i, word := range msgWords {
		msgWords[i] = sclean.AlphaNumOnly(word)
	}
	var outLines []string
	responseCount := 0
	for _, help := range helps {
		if responseCount >= maxCombinedResponses {
			break
		}
		keyword := help.match(msgLower, msgWild, msgWords)
		if keyword == "" {
			continue
		}
		if len(outLines) != 0 {
			outLines = append(outLines, "")
		}
		outLines = append(outLines, help.title(keyword)+":")
		outLines = append(outLines, help.ReplyLines...)
		responseCount++
	}
	return outLines
}

func (help helpData) match(msgLower, msgWild string, msgWords []string) string {
	for _, exclude := range help.Exclude {
		exclude = strings.ToLower(exclude)
		// URLs need their punctuation; existing compact exclusions still work.
		if exclude != "" && (strings.Contains(msgLower, exclude) || strings.Contains(msgWild, exclude)) {
			return ""
		}
	}
	for _, wildcard := range help.Wildcards {
		if wildcard != "" && strings.Contains(msgWild, strings.ToLower(wildcard)) {
			return wildcard
		}
	}
	for _, word := range msgWords {
		if word == "" {
			continue
		}
		for _, keyword := range help.Words {
			if strings.EqualFold(word, keyword) {
				return keyword
			}
		}
	}
	return ""
}

func sendHelpReply(s *discordgo.Session, m *discordgo.MessageCreate, content string) {
	// Keep the check, send, and accounting together so concurrent handlers cannot
	// bypass the limits. A failed send must not consume a user's daily reply.
	replyMu.Lock()
	defer replyMu.Unlock()
	if !checkThrottle(m) {
		return
	}
	reply := &discordgo.MessageSend{
		Content: content,
		Reference: &discordgo.MessageReference{
			MessageID: m.ID,
			ChannelID: m.ChannelID,
			GuildID:   m.GuildID,
		},
	}
	if _, err := s.ChannelMessageSendComplex(m.ChannelID, reply); err != nil {
		cwlog.DoLog(fmt.Sprintf("Could not send help reply: %v", err))
		return
	}
	if !skipThrottle {
		now := time.Now()
		if users[m.Author.ID] == nil {
			users[m.Author.ID] = &userData{id: m.Author.ID}
		}
		users[m.Author.ID].lastSaw = now
		users[m.Author.ID].total++
		lastReply = now
		totalMsgCount++
	}
	cwlog.DoLog(fmt.Sprintf("TRIGGERED:\n%v: %v: %v\nReply: %v", m.ChannelID, m.Author.Username, m.Content, content))
}

func (help helpData) title(fallback string) string {
	if help.Title != "" {
		return help.Title
	}
	return fallback
}

// checkThrottle requires replyMu to be held. It does not consume an allowance.
func checkThrottle(m *discordgo.MessageCreate) bool {
	if skipThrottle {
		return true
	}

	if totalMsgCount >= maxGlobal {
		return false
	}

	if time.Since(lastReply) < throttleGlobal {
		cwlog.DoLog(fmt.Sprintf("global throttled: User: %v, Message: %v", m.Author.ID, m.Content))
		return false
	}
	if user := users[m.Author.ID]; user != nil {
		if user.total >= maxPerUser {
			return false
		}
		if time.Since(user.lastSaw) < throttlePerUser {
			cwlog.DoLog(fmt.Sprintf("user throttled: User: %v, Message: %v", m.Author.ID, m.Content))
			return false
		}
	}

	return true
}
