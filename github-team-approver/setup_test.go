package function

import (
	"os"
	"strconv"
	"testing"

	"github.com/form3tech-oss/go-pact-testing/pacttesting"
)

const (
	envRunPactTests = "RUN_PACT_TESTS"
)

func PactTest(t *testing.T) {
	if v, err := strconv.ParseBool(os.Getenv(envRunPactTests)); err != nil || !v {
		t.Skipf("Skipping because %q is not true", envRunPactTests)
	}
}

func TestMain(m *testing.M) {
	result := m.Run()
	pacttesting.StopMockServers()
	os.Exit(result)
}
