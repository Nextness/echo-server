ARG GO_VERSION=1.26.6

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/echo-server \
    ./cmd/echo

FROM scratch
COPY --from=build --chown=65532:65532 /out/echo-server /echo-server

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/echo-server"]
