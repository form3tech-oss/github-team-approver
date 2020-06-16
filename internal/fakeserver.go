package internal

import (
	"context"
	"fmt"
	"github.com/liamg/waitforhttp"
	"github.com/phayes/freeport"
	"io/ioutil"
	"net/http"
	"time"
)

type FakeServer struct {
	addr         string
	requestStore map[string]string
	server       *http.Server
	mux          http.ServeMux
}

func NewFakeSlackServer() (*FakeServer, error) {
	serverPort, _ := freeport.GetFreePort()
	return &FakeServer{
		addr:         fmt.Sprintf(":%d", serverPort),
		requestStore: map[string]string{},
	}, nil
}

func (f *FakeServer) Start() error {

	mux := http.NewServeMux()

	f.server = &http.Server{
		Addr:    f.addr,
		Handler: mux,
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
		f.requestStore[hookId] = string(b)
		w.WriteHeader(http.StatusOK)
	}
	f.mux.HandleFunc(hookPath, handler)

	return fmt.Sprintf("http://localhost%s%s", f.addr, hookPath)
}

func (f *FakeServer) GetHookRequest(hookId string) string {
	return f.requestStore[hookId]
}






