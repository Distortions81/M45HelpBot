package main

import (
	"sync"
	"time"
)

const (
	maxAttempts          = 50
	throttlePerUser      = time.Hour * 24
	throttleGlobal       = time.Minute * 15
	maxPerUser           = 3
	maxGlobal            = 14
	maxCombinedResponses = 3
	rebootTime           = time.Hour * 24 * 7
)

var (
	/* Discord data */
	discToken    string
	staffRole    string
	guildID      string
	staffChannel string

	replyMu       sync.Mutex
	lastReply     time.Time
	users         = map[string]*userData{}
	totalMsgCount int
)

type helpData struct {
	Title      string   `json:",omitempty"`
	Wildcards  []string `json:",omitempty"`
	Words      []string `json:",omitempty"`
	Exclude    []string `json:",omitempty"`
	ReplyLines []string `json:",omitempty"`
}

type userData struct {
	id      string
	lastSaw time.Time
	total   int
}
