FROM golang:1.23 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ffuf .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /ffuf
COPY --from=builder /app/ffuf /usr/local/bin/ffuf
LABEL org.opencontainers.image.title="ffuf" \
      org.opencontainers.image.description="Fast web fuzzer written in Go" \
      org.opencontainers.image.source="https://github.com/ffuf/ffuf"
ENTRYPOINT ["ffuf"]
