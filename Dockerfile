FROM golang:1.19.5-alpine as build-env
RUN apk add build-base
RUN go install -v github.com/ffuf/ffuf/v2@latest

FROM alpine:3.17.1
RUN apk add --no-cache bind-tools ca-certificates
COPY --from=build-env /go/bin/ffuf /usr/local/bin/ffuf
ENTRYPOINT ["ffuf"]
