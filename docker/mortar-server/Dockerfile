# Build image
FROM golang AS builder

WORKDIR /app

COPY . .

# RUN go install \
#       github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
#       github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
#       github.com/golang/protobuf/protoc-gen-go
# 
# RUN apt update && apt install -y \
#       protobuf-compiler \
#       python3-pip
#
# RUN pip3 install grpcio-tools

RUN make mortar-server

# Runtime image
FROM ubuntu:19.10

COPY --from=builder /app/mortar-server .

CMD ["./mortar-server"]

