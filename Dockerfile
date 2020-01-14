FROM golang:1.13-alpine as builder

RUN apk add --no-cache git gcc libc-dev

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

COPY $PWD/ /go/src/app/
WORKDIR /go/src/app/

RUN go get app && go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o /go/bin/prometheus-digitalocean-sd

# Final image
FROM gcr.io/distroless/base

COPY --from=builder /go/bin/prometheus-digitalocean-sd /prometheus-digitalocean-sd

ENTRYPOINT ["/prometheus-digitalocean-sd"]

