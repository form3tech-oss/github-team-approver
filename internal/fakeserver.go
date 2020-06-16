package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/liamg/waitforhttp"
	"github.com/phayes/freeport"
	"github.com/slack-go/slack"
	"io/ioutil"
	"net/http"
	"time"
)

type FakeServer struct {
	addr         string
	requestStore map[string]slack.WebhookMessage
	server       *http.Server
	mux          *http.ServeMux
}

func NewFakeSlackServer() (*FakeServer, error) {
	serverPort, _ := freeport.GetFreePort()
	return &FakeServer{
		addr:         fmt.Sprintf(":%d", serverPort),
		requestStore: map[string]slack.WebhookMessage{},
		mux:          http.NewServeMux(),
	}, nil
}

func (f *FakeServer) Start() error {

	//f.AddHookEndpoint("1234")
	f.server = &http.Server{
		Addr:    f.addr,
		Handler: f.mux,
	}
	serverCh := make(chan struct{})
	go func() {
		if err := f.server.ListenAndServe(); err != http.ErrServerClosed {
		}
		close(serverCh)
	}()

	return  waitforhttp.Wait(f.server, 10 * time.Second)
}

func (f *FakeServer) Stop() error{
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := f.server.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}


func (f *FakeServer) AddHookEndpoint(hookId string) string {
	hookPath := fmt.Sprintf("/%s", hookId)
	handler := func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		var slackMsg slack.WebhookMessage
		json.Unmarshal(b, &slackMsg)
		f.requestStore[hookId] = slackMsg
		w.WriteHeader(http.StatusOK)
	}
	f.mux.HandleFunc(hookPath, handler)

	return fmt.Sprintf("http://localhost%s%s", f.addr, hookPath)
}

func (f *FakeServer) GetHookRequest(hookId string) slack.WebhookMessage {
	return f.requestStore[hookId]
}





