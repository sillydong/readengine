package main

import (
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	t.Log(path.Dir(os.Args[0]))
	t.Log(filepath.Dir(os.Args[0]))
}
