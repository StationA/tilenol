package tilenol

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Disable logging
	Logger.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}
