package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

type replyRecorder struct {
	mu      sync.Mutex
	replies []discordgo.MessageSend
	status  int
}

func (r *replyRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var reply discordgo.MessageSend
	if err := json.NewDecoder(req.Body).Decode(&reply); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replies = append(r.replies, reply)
	body := `{"id":"reply"}`
	if r.status != http.StatusOK {
		body = `{"code":50013,"message":"Missing Permissions"}`
	}
	return &http.Response{
		StatusCode: r.status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func setupMessageTest(t *testing.T) (*discordgo.Session, *replyRecorder) {
	t.Helper()
	oldHelps, oldUsers := helpsList, users
	oldGuild, oldRole, oldChannel := guildID, staffRole, staffChannel
	oldSkip, oldLast, oldTotal := skipThrottle, lastReply, totalMsgCount
	t.Cleanup(func() {
		helpsList, users = oldHelps, oldUsers
		guildID, staffRole, staffChannel = oldGuild, oldRole, oldChannel
		skipThrottle, lastReply, totalMsgCount = oldSkip, oldLast, oldTotal
	})
	helpsList = []HelpsListData{{Name: "main", Data: []helpData{{
		Title: "Membership", Words: []string{"register"}, ReplyLines: []string{"Registration instructions."},
	}}}}
	users = map[string]*userData{}
	guildID, staffRole, staffChannel = "guild", "", ""
	skipThrottle, lastReply, totalMsgCount = true, time.Time{}, 0
	s, err := discordgo.New("Bot offline-test")
	if err != nil {
		t.Fatal(err)
	}
	s.State.User = &discordgo.User{ID: "bot", Bot: true}
	recorder := &replyRecorder{status: http.StatusOK}
	s.Client = &http.Client{Transport: recorder}
	s.MaxRestRetries = 0
	return s, recorder
}

func testMessage(content, user string) *discordgo.MessageCreate {
	return &discordgo.MessageCreate{Message: &discordgo.Message{
		ID: "message", GuildID: "guild", ChannelID: "channel", Content: content,
		Author: &discordgo.User{ID: user, Username: user}, Member: &discordgo.Member{},
	}}
}

func TestMessageMatching(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		words, wilds  []string
		excludes      []string
		want          bool
	}{
		{name: "plain", message: "How do I register?", words: []string{"register"}, want: true},
		{name: "multiline", message: "How do I\nregister?", words: []string{"register"}, want: true},
		{name: "tabs", message: "How do I\tregister?", words: []string{"register"}, want: true},
		{name: "unicode whitespace", message: "How do I\u00a0register?", words: []string{"register"}, want: true},
		{name: "markdown", message: "**How** do I `register`?", words: []string{"register"}, want: true},
		{name: "whole words", message: "unregistered", words: []string{"register"}},
		{name: "duplicate words", message: "register register", words: []string{"register", "REGISTER", "register"}, want: true},
		{name: "wildcard", message: "Can we reset the map?", wilds: []string{"resetmap"}, want: true},
		{name: "wildcard case", message: "Can we reset the map?", wilds: []string{"RESETMAP"}, want: true},
		{name: "both trigger types", message: "register", words: []string{"register"}, wilds: []string{"register"}, want: true},
		{name: "https exclusion", message: "register at https://example.org", words: []string{"register"}, excludes: []string{"https://"}},
		{name: "http exclusion", message: "register at HTTP://example.org", words: []string{"register"}, excludes: []string{"http://"}},
		{name: "wildcard exclusion", message: "reset the map https://example.org", wilds: []string{"resetmap"}, excludes: []string{"https://"}},
		{name: "compact exclusion", message: "I already registered", words: []string{"registered"}, excludes: []string{"alreadyregistered"}},
		{name: "scheme punctuation matters", message: "Does register use https?", words: []string{"register"}, excludes: []string{"https://"}, want: true},
		{name: "empty wildcard", message: "hello", wilds: []string{""}},
		{name: "empty word", message: "hello !", words: []string{""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, recorder := setupMessageTest(t)
			help := &helpsList[0].Data[0]
			help.Words, help.Wildcards, help.Exclude = tc.words, tc.wilds, tc.excludes
			MessageCreate(s, testMessage(tc.message, "user"))
			if !tc.want {
				if len(recorder.replies) != 0 {
					t.Fatalf("unexpected reply: %q", recorder.replies[0].Content)
				}
				return
			}
			if len(recorder.replies) != 1 {
				t.Fatalf("got %d replies, want 1", len(recorder.replies))
			}
			if got := recorder.replies[0].Content; got != "Membership:\nRegistration instructions." {
				t.Errorf("unexpected or duplicate response: %q", got)
			}
			if ref := recorder.replies[0].Reference; ref == nil || ref.MessageID != "message" || ref.ChannelID != "channel" || ref.GuildID != "guild" {
				t.Errorf("reply lost the original message reference: %+v", ref)
			}
		})
	}
}

func TestCombinedResponsesKeepOrderAndLimit(t *testing.T) {
	s, recorder := setupMessageTest(t)
	helpsList[0].Data = nil
	for i := 0; i < maxCombinedResponses+1; i++ {
		helpsList[0].Data = append(helpsList[0].Data, helpData{
			Title: fmt.Sprintf("Topic %d", i), Words: []string{"register", "REGISTER"}, ReplyLines: []string{"Instructions."},
		})
	}
	MessageCreate(s, testMessage("register", "user"))
	if len(recorder.replies) != 1 {
		t.Fatalf("got %d replies, want 1", len(recorder.replies))
	}
	var want []string
	for i := 0; i < maxCombinedResponses; i++ {
		want = append(want, fmt.Sprintf("Topic %d:\nInstructions.", i))
	}
	if got := recorder.replies[0].Content; got != strings.Join(want, "\n\n") {
		t.Errorf("combined reply = %q, want %q", got, strings.Join(want, "\n\n"))
	}
}

func TestMessageIgnoresUnsupportedEvents(t *testing.T) {
	for _, name := range []string{"nil event", "nil message", "nil author", "DM", "other guild", "bot", "self", "webhook"} {
		t.Run(name, func(t *testing.T) {
			s, recorder := setupMessageTest(t)
			m := testMessage("register", "user")
			switch name {
			case "nil event":
				m = nil
			case "nil message":
				m.Message = nil
			case "nil author":
				m.Author = nil
			case "DM":
				m.GuildID, m.Member = "", nil
			case "other guild":
				m.GuildID, m.Member = "other", nil
			case "bot":
				m.Author.Bot = true
			case "self":
				m.Author.ID = "bot"
			case "webhook":
				m.WebhookID = "webhook"
			}
			MessageCreate(s, m)
			if len(recorder.replies) != 0 {
				t.Fatalf("got %d replies for an unsupported event", len(recorder.replies))
			}
		})
	}
}

func TestStaffHelpStaysInConfiguredChannel(t *testing.T) {
	for _, tc := range []struct {
		name, channel string
		member        *discordgo.Member
		want          string
	}{
		{name: "staff channel", channel: "staff-channel", member: &discordgo.Member{Roles: []string{"staff"}}, want: "Staff instructions."},
		{name: "staff elsewhere", channel: "channel", member: &discordgo.Member{Roles: []string{"staff"}}},
		{name: "nonstaff", channel: "channel", member: &discordgo.Member{}, want: "Registration instructions."},
		{name: "missing member", channel: "channel", want: "Registration instructions."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, recorder := setupMessageTest(t)
			staffRole, staffChannel = "staff", "staff-channel"
			helpsList = append(helpsList, HelpsListData{Name: "staff", Data: []helpData{{
				Title: "Staff", Words: []string{"register"}, ReplyLines: []string{"Staff instructions."},
			}}})
			m := testMessage("register", "user")
			m.ChannelID, m.Member = tc.channel, tc.member
			MessageCreate(s, m)
			if tc.want == "" {
				if len(recorder.replies) != 0 {
					t.Fatal("staff reply escaped the configured channel")
				}
			} else if len(recorder.replies) != 1 || !strings.Contains(recorder.replies[0].Content, tc.want) {
				t.Errorf("incorrect help audience: %+v", recorder.replies)
			}
		})
	}
}

func TestThrottleCooldowns(t *testing.T) {
	s, recorder := setupMessageTest(t)
	skipThrottle = false
	MessageCreate(s, testMessage("register", "first"))
	MessageCreate(s, testMessage("register", "second"))
	if len(recorder.replies) != 1 || totalMsgCount != 1 || lastReply.IsZero() {
		t.Fatalf("global cooldown not recorded: sends=%d total=%d last=%v", len(recorder.replies), totalMsgCount, lastReply)
	}
	lastReply = time.Now().Add(-throttleGlobal - time.Second)
	MessageCreate(s, testMessage("register", "first"))
	if len(recorder.replies) != 1 {
		t.Fatal("user cooldown did not apply after the global cooldown expired")
	}
	MessageCreate(s, testMessage("register", "second"))
	if len(recorder.replies) != 2 {
		t.Fatal("new user was blocked after the global cooldown expired")
	}
	lastReply = time.Now().Add(-throttleGlobal - time.Second)
	users["first"].lastSaw = time.Now().Add(-throttlePerUser - time.Second)
	MessageCreate(s, testMessage("register", "first"))
	if len(recorder.replies) != 3 || users["first"].total != 2 {
		t.Fatal("user was blocked after both cooldowns expired")
	}
}

func TestThrottleExactCaps(t *testing.T) {
	for _, scope := range []string{"global", "user"} {
		t.Run(scope, func(t *testing.T) {
			s, recorder := setupMessageTest(t)
			skipThrottle = false
			if scope == "global" {
				totalMsgCount = maxGlobal - 1
			} else {
				users["user"] = &userData{total: maxPerUser - 1, lastSaw: time.Now().Add(-2 * throttlePerUser)}
			}
			MessageCreate(s, testMessage("register", "user"))
			lastReply = time.Now().Add(-2 * throttleGlobal)
			users["user"].lastSaw = time.Now().Add(-2 * throttlePerUser)
			MessageCreate(s, testMessage("register", "user"))
			if len(recorder.replies) != 1 {
				t.Fatalf("%s cap allowed %d sends at the boundary, want 1", scope, len(recorder.replies))
			}
		})
	}
}

func TestFailedReplyDoesNotConsumeThrottle(t *testing.T) {
	s, recorder := setupMessageTest(t)
	skipThrottle = false
	recorder.status = http.StatusForbidden
	MessageCreate(s, testMessage("register", "user"))
	if totalMsgCount != 0 || !lastReply.IsZero() || len(users) != 0 {
		t.Fatal("failed send consumed the cooldown or reply allowance")
	}
	recorder.status = http.StatusOK
	MessageCreate(s, testMessage("register", "user"))
	if len(recorder.replies) != 2 || totalMsgCount != 1 {
		t.Fatal("user could not retry after a failed send")
	}
}

func TestConcurrentMessagesRespectGlobalCooldown(t *testing.T) {
	s, recorder := setupMessageTest(t)
	skipThrottle = false
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		workers.Go(func() {
			<-start
			MessageCreate(s, testMessage("register", fmt.Sprintf("user-%d", i)))
		})
	}
	close(start)
	workers.Wait()
	if len(recorder.replies) != 1 || totalMsgCount != 1 || len(users) != 1 {
		t.Errorf("concurrent messages bypassed the cooldown: sends=%d total=%d users=%d", len(recorder.replies), totalMsgCount, len(users))
	}
}

func TestTestModeBypassesThrottle(t *testing.T) {
	s, recorder := setupMessageTest(t)
	totalMsgCount, lastReply = maxGlobal, time.Now()
	users["user"] = &userData{total: maxPerUser, lastSaw: time.Now()}
	MessageCreate(s, testMessage("register", "user"))
	MessageCreate(s, testMessage("register", "user"))
	if len(recorder.replies) != 2 || totalMsgCount != maxGlobal || users["user"].total != maxPerUser {
		t.Fatal("test mode failed to bypass throttling without consuming allowances")
	}
}

func TestStartupWithoutTokenOrAfterCancellation(t *testing.T) {
	if bot, err := startbot(context.Background(), ""); err == nil || bot != nil {
		t.Fatal("missing token did not fail startup")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if bot, err := startbot(ctx, "offline-test"); !errors.Is(err, context.Canceled) || bot != nil {
		t.Fatalf("canceled startup returned session=%v error=%v", bot, err)
	}
}
