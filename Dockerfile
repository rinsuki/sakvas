FROM golang:1.19.3-alpine

WORKDIR /app
COPY go.sum go.mod ./
RUN go mod download 
COPY *.go .
RUN go build -o /sakvas

ENV GIN_MODE release
EXPOSE 3000

WORKDIR /
CMD ["/sakvas"]
