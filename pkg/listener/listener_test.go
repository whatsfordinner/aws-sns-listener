package listener

import (
	"io"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	logger.SetOutput(io.Discard)
	os.Exit(m.Run())
}
