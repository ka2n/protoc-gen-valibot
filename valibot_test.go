package protocgenvalibot

import (
	"flag"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tenntenn/golden"
)

var flagUpdate bool

func init() {
	flag.BoolVar(&flagUpdate, "update", false, "update .golden files")
}

func TestValibot(t *testing.T) {
	dir := t.TempDir()

	// Run `buf generate`
	cmd := exec.Command("buf", "generate", "-o", dir)
	cmd.Dir = "./testdata/sample"
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)

	got := golden.Txtar(t, dir)
	if diff := golden.Check(t, flagUpdate, "testdata/sample", "output", got); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}
