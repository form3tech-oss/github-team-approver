package function

import (
	"context"
	"os"

	handler "github.com/openfaas-incubator/go-function-sdk"
	log "github.com/sirupsen/logrus"
)

func getAppNameOrDefault() string {
	if v := os.Getenv(envAppName); v != "" {
		return v
	}
	return defaultAppName
}

func getLogger(ctx context.Context) *log.Entry {
	l, ok := ctx.Value(contextKeyLogger).(*log.Entry)
	if !ok {
		return log.NewEntry(log.StandardLogger())
	}
	return l
}

func newRequestContext(req handler.Request) context.Context {
	return context.WithValue(context.Background(), contextKeyLogger, log.WithFields(log.Fields{
		logFieldServiceName: getAppNameOrDefault(),
		logFieldDeliveryID:  req.Header.Get(httpHeaderXGithubDelivery),
		logFieldEventType:   req.Header.Get(httpHeaderXGithubEvent),
	}))
}

func updateRequestContext(ctx context.Context, eventType string, event event) context.Context {
	return context.WithValue(ctx, contextKeyLogger, ctx.Value(contextKeyLogger).(*log.Entry).WithFields(log.Fields{
		logFieldEventType: eventType,
		logFieldRepo:      event.GetRepo().GetFullName(),
		logFieldPR:        event.GetPullRequest().GetNumber(),
	}))
}
