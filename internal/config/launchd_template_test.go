package config

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchdLogsUseAbsoluteUserLibraryPaths(t *testing.T) {
	data, err := os.ReadFile("../../deploy/launchd/com.cameronsjo.kanban-mcp.plist")
	if err != nil {
		t.Fatal(err)
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	key := ""
	paths := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "key" {
			if err := decoder.DecodeElement(&key, &start); err != nil {
				t.Fatal(err)
			}
		} else if start.Name.Local == "string" && (key == "StandardOutPath" || key == "StandardErrorPath") {
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				t.Fatal(err)
			}
			paths[key] = value
			key = ""
		}
	}
	for _, key := range []string{"StandardOutPath", "StandardErrorPath"} {
		value := paths[key]
		if !filepath.IsAbs(value) || !strings.Contains(value, "/Library/Logs/") || strings.ContainsAny(value, "~$") {
			t.Errorf("%s=%q must use an absolute Library/Logs path without shell expansion", key, value)
		}
	}
}
