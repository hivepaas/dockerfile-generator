FROM docker.io/library/golang:1.23 AS builder

ARG GOPROXY=direct
WORKDIR /app
COPY . .
RUN GOPROXY=${GOPROXY} CGO_ENABLED=0 go build -o dockerfile-gen cmd/dockerfile-gen/main.go

FROM docker.io/library/alpine:3.19.1

WORKDIR /app
COPY --from=builder /app/dockerfile-gen /usr/local/bin

CMD [ "dockerfile-gen" ]
