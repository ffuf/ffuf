FROM golang:1.15-alpine AS builder
COPY . /app
WORKDIR /app
RUN go get
RUN go build -o ffuf

FROM alpine
RUN adduser --home /app --shell /bin/sh --disabled-password appuser
COPY --from=builder --chown=appuser:appuser /app/ffuf /app
USER appuser

WORKDIR /app
ENTRYPOINT ["/app/ffuf"]
CMD ["-h"]
