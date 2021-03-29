FROM golang:1.16.2-alpine

WORKDIR /app
COPY go.sum go.mod ./
RUN go mod download 
COPY *.go .
RUN go build -o /sakvas

EXPOSE 3000

WORKDIR /
CMD ["/sakvas"]