FROM golang:1.11

MAINTAINER int-ono

WORKDIR /go/src/app

RUN go get 

CMD ["http_exporter", "-a", ":8888"]
