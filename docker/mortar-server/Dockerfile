# Build image
FROM golang AS builder

WORKDIR /app

COPY main.go .
COPY go.mod .
COPY go.sum .
COPY internal internal
COPY Makefile .
RUN go build -o mortar-server

# Runtime image
FROM ubuntu:19.10

COPY --from=builder /app/mortar-server .

CMD ["./mortar-server"]

