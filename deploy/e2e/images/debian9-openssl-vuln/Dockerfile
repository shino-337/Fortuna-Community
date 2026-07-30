FROM debian:9

RUN apt-get update && \
    apt-get install -y openssl curl && \
    rm -rf /var/lib/apt/lists/*

CMD ["sleep","3600"]
