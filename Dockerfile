FROM golang:1.21 as builder
COPY . /go/src/ffuf
WORKDIR /go/src/ffuf
RUN CGO_ENABLED=0 go build .

FROM scratch
COPY --from=builder /go/src/ffuf/ffuf /bin/ffuf
CMD ["/bin/ffuf"]
