# Multi-stage build: compile Rust cdylib and NGINX dynamic module, assemble runnable image
#
# Resulting image runs NGINX with the Rust-backed EPP module loaded.
# The module exposes the directive:
#   epp_get_endpoint on;
#
# Notes:
# - This POC links the C module against a Rust cdylib. We set LD_LIBRARY_PATH
#   so the runtime can resolve the Rust FFI library.
# - Module builds against a specific NGINX OSS version; adjust NGINX_VERSION if needed.

ARG NGINX_VERSION=1.25.5

# Stage 1: Build Rust cdylib
FROM rust:1.82-slim AS rust-builder
WORKDIR /work
# Install minimal build deps if needed by crates (prost/tonic compile-time only in later iterations)
RUN apt-get update && apt-get install -y --no-install-recommends pkg-config && rm -rf /var/lib/apt/lists/*
# Copy Rust crate
COPY dev/rust-nginx-epp/Cargo.toml dev/rust-nginx-epp/Cargo.toml
COPY dev/rust-nginx-epp/src/ dev/rust-nginx-epp/src/
# Build release cdylib
WORKDIR /work/dev/rust-nginx-epp
RUN cargo build --release
# Export the dynamic library (name is platform-dependent; we expect libngx_http_rust_epp.so on Linux)
RUN mkdir -p /out && \
    cp target/release/libngx_http_rust_epp.so /out/ || \
    (echo "Note: lib name may differ; listing:" && ls -la target/release && exit 1)

# Stage 2: Build NGINX + dynamic module
FROM debian:bookworm-slim AS nginx-builder
ARG NGINX_VERSION
ENV DEBIAN_FRONTEND=noninteractive
# Build dependencies for NGINX
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential ca-certificates curl git \
    libpcre3-dev zlib1g-dev libssl-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /work
# Copy module sources and Rust FFI lib
COPY dev/rust-nginx-epp/c/ /work/dev/rust-nginx-epp/c/
COPY --from=rust-builder /out/libngx_http_rust_epp.so /work/lib/
# Fetch and unpack NGINX source
RUN curl -fsSL "http://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz" -o nginx.tar.gz && \
    tar -xzf nginx.tar.gz && rm nginx.tar.gz
WORKDIR /work/nginx-${NGINX_VERSION}
# Configure NGINX to build dynamic module; pass the Rust lib dir for linking
ENV NGX_RUST_EPP_LIB_DIR=/work/lib
RUN ./configure \
    --with-compat \
    --with-http_ssl_module \
    --add-dynamic-module=/work/dev/rust-nginx-epp/c \
 && make -j"$(nproc)" \
 && make modules \
 && make install
# Copy built module artifact to a known location
RUN mkdir -p /out/modules && \
    if [ -f objs/ngx_http_rust_epp_module.so ]; then cp objs/ngx_http_rust_epp_module.so /out/modules/; else cp /usr/local/nginx/modules/ngx_http_rust_epp_module.so /out/modules/; fi && \
    mkdir -p /out/sbin && cp /usr/local/nginx/sbin/nginx /out/sbin/ && \
    mkdir -p /out/conf && cp -r /usr/local/nginx/conf/* /out/conf/ && \
    mkdir -p /out/html && cp -r /usr/local/nginx/html/* /out/html/

# Stage 3: Runtime image
FROM debian:bookworm-slim
ARG NGINX_VERSION
ENV LD_LIBRARY_PATH=/etc/nginx/modules
# Minimal runtime deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libpcre3 zlib1g libssl3 \
 && rm -rf /var/lib/apt/lists/*
# Install NGINX runtime files
COPY --from=nginx-builder /out/sbin/nginx /usr/sbin/nginx
COPY --from=nginx-builder /out/conf/ /etc/nginx/
COPY --from=nginx-builder /out/html/ /usr/share/nginx/html/
# Install module and Rust FFI library
COPY --from=nginx-builder /out/modules/ngx_http_rust_epp_module.so /etc/nginx/modules/
COPY --from=rust-builder /out/libngx_http_rust_epp.so /etc/nginx/modules/
# Provide POC nginx.conf
COPY dev/rust-nginx-epp/nginx.conf /etc/nginx/nginx.conf
# Create legacy NGINX prefix paths expected by the compiled binary and redirect logs to stderr
RUN mkdir -p /usr/local/nginx/logs /usr/local/nginx/conf /etc/nginx/logs && \
    ln -sf /dev/stderr /usr/local/nginx/logs/error.log && \
    ln -sf /etc/nginx/nginx.conf /usr/local/nginx/conf/nginx.conf
# Expose port and default command
EXPOSE 80
STOPSIGNAL SIGTERM
CMD ["/usr/sbin/nginx", "-p", "/etc/nginx", "-c", "/etc/nginx/nginx.conf", "-e", "/dev/stderr", "-g", "daemon off;"]
