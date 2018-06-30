package tinybiome

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	files, _ := filepath.Glob("*.yml")
	for _, name := range files {
		conf, err := ConfigFromFile(name)
		if err != nil {
			t.Error("error occurred", err)
		}
		t.Log(name)
		t.Logf("%#v", conf)
		t.Log(conf)
	}
}
