FROM golang:latest AS build-env
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.* /src/
RUN go mod download
COPY . .
RUN go build -a -o ffuf -ldflags="-s -w"

FROM alpine:latest

RUN apk add --no-cache ca-certificates \
    && rm -rf /var/cache/*

USER nobody
WORKDIR /app

COPY --from=build-env /src/ffuf .

CMD [ "/app/ffuf" ]
