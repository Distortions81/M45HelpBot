package main

import (
	"M45HelpBot/cwlog"
	"bytes"
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

	err = json.Unmarshal(file, &helpsList)

	if err != nil {
		cwlog.DoLog("Error: readHelps: Unable to unmashal helps file.")
		return false
	}

	helpsCount := 0
	for _, helpsType := range helpsList {
		helpsCount += len(helpsType.Data)
	}

	buf := fmt.Sprintf("Loaded %v helps types, and %v helps.", len(helpsList), helpsCount)
	cwlog.DoLog(buf)

	return true
}

func writeHelps() {
	outbuf := new(bytes.Buffer)
	enc := json.NewEncoder(outbuf)
	enc.SetIndent("", "\t")

	if err := enc.Encode(helpsList); err != nil {
		cwlog.DoLog("writeHelps: enc.Encode failure")
		return
	}

	err := os.WriteFile(helpsFile, outbuf.Bytes(), 0755)

	if err != nil {
		cwlog.DoLog("Error: writeHelps: Unable to write the helps file.")
	}
}
