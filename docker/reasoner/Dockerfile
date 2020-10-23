FROM rust AS builder
WORKDIR /app

COPY reasoner .
RUN cargo build --release

# Runtime image
FROM debian:buster-slim

COPY --from=builder /app/target/release/reasoner .
ENV RUST_LOG=info
CMD ["./reasoner"]
