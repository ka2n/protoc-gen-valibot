package protocgenvalibot

import (
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestValibot(t *testing.T) {
	// Run `buf generate`
	cmd := exec.Command("buf", "generate")
	cmd.Dir = "./testdata/sample"
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err)

	// ./output/sample.valibot.ts should be identical to ./expected/sample.valibot.ts
	expected, err := os.ReadFile("./testdata/sample/expected/sample.valibot.ts")
	require.NoError(t, err)

	actual, err := os.ReadFile("./testdata/sample/output/sample.valibot.ts")
	require.NoError(t, err)

	t.Logf("actual: %s", string(actual))

	if diff := cmp.Diff(string(expected), string(actual)); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}
