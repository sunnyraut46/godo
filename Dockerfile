FROM golang:1.13
COPY . /godo
WORKDIR /godo
RUN go build ./test/e2e
