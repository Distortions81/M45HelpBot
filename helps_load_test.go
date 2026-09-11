package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInvalidHelpFilePreservesLoadedCatalog(t *testing.T) {
	setupMessageTest(t)
	oldPath := helpsFile
	t.Cleanup(func() { helpsFile = oldPath })
	helpsFile = filepath.Join(t.TempDir(), "helps.json")
	if err := os.WriteFile(helpsFile, []byte(`[{"Name":"broken","Data":[{"Words":[1]}]}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if readHelps() {
		t.Fatal("invalid help file was accepted")
	}
	if len(helpsList) != 1 || helpsList[0].Name != "main" || helpsList[0].Data[0].Title != "Membership" {
		t.Fatalf("invalid help file damaged the loaded catalog: %+v", helpsList)
	}
}

func TestShutdownLeavesHelpFileUntouched(t *testing.T) {
	setupMessageTest(t)
	t.Chdir(t.TempDir())
	oldPath := helpsFile
	t.Cleanup(func() { helpsFile = oldPath })
	helpsFile = "helps.json"
	// Unknown metadata and custom formatting must survive startup and shutdown.
	content := "[ {\"Name\":\"main\", \"EditorNote\":\"keep this\", \"Data\":[]} ]\n"
	if err := os.WriteFile(helpsFile, []byte(content), 0640); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := run(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(helpsFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("help file was rewritten: %s", got)
	}
	info, err := os.Stat(helpsFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("help file permissions changed to %v", info.Mode().Perm())
	}
}
