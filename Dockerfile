FROM golang:1.16-alpine as build
RUN apk --no-cache add git
ENV GO111MODULE on
RUN go get -v github.com/ffuf/ffuf

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=build /go/bin/ffuf /bin/ffuf
ENV HOME /
ENTRYPOINT ["/bin/ffuf"]
