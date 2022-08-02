# Dockerfile implements Rust-ready image with tarpaulin installed

FROM rust:1.62.0-buster

# Check cargo version
RUN cargo --version

RUN cargo install cargo-tarpaulin --version 0.20.1

VOLUME [ "/code" ]
WORKDIR /code

# Default argument when none is provided
CMD ["."]
