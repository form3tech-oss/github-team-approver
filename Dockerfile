FROM golang@sha256:6b6fd1071edb52b61f79aec51281c55050f58dd58e2080b4e24556607c98086f AS build-env # pinned to 1.20.5-alpine3.18 -> https://hub.docker.com/layers/library/golang/1.20.5-alpine3.18/images/sha256-6b6fd1071edb52b61f79aec51281c55050f58dd58e2080b4e24556607c98086f?context=explore
ARG DEBUG
ENV SRCROOT /go/src/github.com/form3tech/github-team-approver
WORKDIR $SRCROOT
COPY go.mod go.sum ./
RUN go mod download
COPY ./cmd/github-team-approver ./cmd/github-team-approver
COPY ./internal/ ./internal/
RUN mkdir /build
RUN --mount=type=cache,target=/root/.cache/go-build,id=github-team-approver \
    if [ "$DEBUG" = "1" ]; then \
        CGO_ENABLED=1 go build -gcflags='all=-N -l' -o /github-team-approver -race -v ./cmd/github-team-approver; \
    else \
        CGO_ENABLED=0 go build -ldflags='-d -s -w' -o /github-team-approver -tags netgo -v ./cmd/github-team-approver; \
    fi

FROM gcr.io/distroless/base:nonroot
ARG APP_NAME
COPY --from=build-env /github-team-approver ./github-team-approver
CMD ["./github-team-approver"]
