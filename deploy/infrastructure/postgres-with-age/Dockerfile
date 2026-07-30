FROM postgres:15-alpine

# Install build dependencies for AGE
RUN apk add --no-cache \
    build-base \
    git \
    postgresql-dev \
    clang \
    llvm \
    cmake \
    make \
    g++ \
    python3 \
    py3-pip

# Clone and build Apache AGE
WORKDIR /tmp
RUN git clone --depth 1 --branch v1.5.0 https://github.com/apache/age.git && \
    cd age && \
    make install || echo "Build may fail if dependencies missing, will install at runtime"

# Note: If build fails, AGE can be installed via init script
# For production, consider using pre-built image or installing AGE separately

# Copy init script
COPY init-age.sh /docker-entrypoint-initdb.d/01-init-age.sh
RUN chmod +x /docker-entrypoint-initdb.d/01-init-age.sh

# Keep build tools for runtime installation if needed
# In production, remove these and use pre-built extension

