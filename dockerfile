ARG GO_VERSION=1.26.5

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/echo-service \
    ./cmd/echo

FROM scratch
COPY --from=build --chown=65532:65532 /out/echo-service /echo-service

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/echo-service"]
