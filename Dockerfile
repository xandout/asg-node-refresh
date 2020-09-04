FROM golang:1.14 as builder
ENV GO111MODULE=on

WORKDIR /go/src/app

ADD go.mod go.sum ./
RUN go get -d ./...

COPY . .

RUN go build -ldflags="-s -w" -o /go/bin/asg-node-refresh

FROM golang:1.14
COPY --from=builder /go/bin/asg-node-refresh /go/bin
CMD ["asg-node-refresh"]