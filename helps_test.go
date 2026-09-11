package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestHelpEntriesAreComplete(t *testing.T) {
	file, err := os.ReadFile("helps.json")
	if err != nil {
		t.Fatalf("read helps.json: %v", err)
	}

	var helpLists []HelpsListData
	if err := json.Unmarshal(file, &helpLists); err != nil {
		t.Fatalf("parse helps.json: %v", err)
	}

	for _, helpList := range helpLists {
		for index, help := range helpList.Data {
			context := fmt.Sprintf("%s[%d]", helpList.Name, index)
			if strings.TrimSpace(help.Title) == "" {
				t.Errorf("%s: missing reply title", context)
			}
			if len(help.Wildcards) == 0 && len(help.Words) == 0 {
				t.Errorf("%s: missing triggers", context)
			}
			if len(help.ReplyLines) == 0 {
				t.Errorf("%s: missing reply", context)
			}
		}
	}
}

func TestHelpTitleFallsBackToMatchedKeyword(t *testing.T) {
	if got := (helpData{}).title("register"); got != "register" {
		t.Errorf("title() = %q, want %q", got, "register")
	}
}
