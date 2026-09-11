package main

import (
	"M45HelpBot/cwlog"
	"encoding/json"
	"fmt"
	"os"
)

var helpsList []HelpsListData

type HelpsListData struct {
	Name string
	Data []helpData
}

func readHelps() bool {
	file, err := os.ReadFile(helpsFile)

	if err != nil {
		cwlog.DoLog(err.Error())
		return false
	}

	var loaded []HelpsListData
	err = json.Unmarshal(file, &loaded)

	if err != nil {
		cwlog.DoLog(fmt.Sprintf("Error: readHelps: Unable to unmarshal helps file: %v", err))
		return false
	}

	helpsList = loaded
	helpsCount := 0
	for _, helpsType := range helpsList {
		helpsCount += len(helpsType.Data)
	}

	buf := fmt.Sprintf("Loaded %v helps types, and %v helps.", len(helpsList), helpsCount)
	cwlog.DoLog(buf)

	return true
}
