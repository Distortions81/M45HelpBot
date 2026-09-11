package sclean

import "testing"

func TestStripControlPreservesText(t *testing.T) {
	if got := StripControl("hello\nworld\t!\x00\x7f café"); got != "helloworld! café" {
		t.Errorf("StripControl() = %q", got)
	}
}

func TestRemoveDiscordMarkdown(t *testing.T) {
	for _, input := range []string{"`register`", "```register```", "**__~~register~~__**"} {
		if got := RemoveDiscordMarkdown(input); got != "register" {
			t.Errorf("RemoveDiscordMarkdown(%q) = %q", input, got)
		}
	}
}
