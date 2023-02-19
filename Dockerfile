FROM golang:1.19 AS build
WORKDIR /go/src/myhttpserver
COPY ["myhttpserver.go", "metrics", "config.json", "go.mod", "go.sum", "./"]

ENV CGO_ENABLED=0   \
    GO111MODULE=on  \
    GOOS=linux      \
    GOARCH=amd64
#ENV GOPROXY=https://goproxy.cn,direct
RUN go build -installsuffix cgo -o /go/bin/myhttpserver myhttpserver.go

FROM alpine:3.17.0
COPY --from=build /go/bin/myhttpserver /myhttpserver
ENV VERSION=1.0
EXPOSE 8888
ENTRYPOINT ["/myhttpserver"]
