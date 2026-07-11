FROM golang:1.22-alpine

RUN apk add --no-cache gcc musl-dev
WORKDIR /app