FROM golang:1.19 AS build
WORKDIR /app
COPY agent/ ./agent/
COPY build.sh go.mod go.sum ./
RUN go mod download && go mod verify
RUN ./build.sh

FROM alpine:3.17
RUN apk update && apk upgrade && apk add bash libc6-compat
WORKDIR /app
COPY --from=build /app/redpanda-edge-agent ./
CMD ./redpanda-edge-agent -config /etc/redpanda/agent.yaml -loglevel debug

