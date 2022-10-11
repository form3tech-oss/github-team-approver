package stages

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/form3tech-oss/github-team-approver/internal/api"
	"github.com/stretchr/testify/require"

	"github.com/sirupsen/logrus"

	"github.com/phayes/freeport"
)

const (
	maxTry    = 5
	sleepTime = 5
)

type AppServer struct {
	t        *testing.T
	ready    chan struct{}
	shutdown chan os.Signal
	stopped  chan struct{}

	healthCheckAttempts int

	url string
}

func NewAppServer(t *testing.T) *AppServer {
	return &AppServer{
		t:     t,
		ready: make(chan struct{}),
		// As we are not processing incoming signals
		// we don't need to buffer shutdown chan
		shutdown: make(chan os.Signal),
		stopped:  make(chan struct{}),

		healthCheckAttempts: maxTry,
	}
}

func (a *AppServer) Shutdown() {
	a.shutdown <- syscall.SIGINT
	<-a.stopped
	logrus.Info("shutdown complete")
}

func (a *AppServer) Start() error {
	serverPort, _ := freeport.GetFreePort()
	bindAddress := fmt.Sprintf("localhost:%d", serverPort)

	u, err := url.Parse(fmt.Sprintf("http://%s", bindAddress))
	require.NoError(a.t, err)
	a.url = u.String()

	go func() {
		// blocking until ListenAndServe returns

		api.Start(bindAddress, a.shutdown, a.ready)
		close(a.stopped)
	}()

	// ListenAndServe takes longer to setup listening socket and receive
	// than it takes for us to receive on channel and sending requests in our test cases
	// hence we introduce health check before returning on start
	<-a.ready
	err = a.waitForHealth()
	if err != nil {
		logrus.WithError(err).
			WithField("url", a.url).
			Fatal("app not ready")
	}

	return nil
}

func (a *AppServer) URL() string { return a.url }
func (a *AppServer) waitForHealth() error {
	for i := 0; i <= a.healthCheckAttempts; i++ {
		if err := a.checkHealth(); err != nil {
			time.Sleep(sleepTime * time.Millisecond)
			continue
		}
		logrus.Info("server health ok")
		return nil
	}
	return fmt.Errorf("server health not ready")
}

func (a *AppServer) checkHealth() error {
	resp, err := http.Get(fmt.Sprintf("%s/health", a.url))
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			require.NoError(a.t, err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("not ok")
	}

	return nil
}
