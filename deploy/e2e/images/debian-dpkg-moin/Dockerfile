FROM debian:10-slim

# Minimal filesystem to let KSAM SBOM extractor find dpkg packages
# We intentionally do NOT install anything via apt.

RUN mkdir -p /var/lib/dpkg
COPY status /var/lib/dpkg/status

CMD ["sh", "-c", "echo 'KSAM E2E dpkg status image (debian base)' && sleep 3600"]


