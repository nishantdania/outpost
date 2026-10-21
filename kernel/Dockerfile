# Build-time tooling only; no compiler/container engine is installed on VM hosts.
FROM ubuntu@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517
COPY images/default/files/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN sed -i 's|http://archive.ubuntu.com/ubuntu/|https://snapshot.ubuntu.com/ubuntu/20260823T000000Z/|g; s|http://security.ubuntu.com/ubuntu/|https://snapshot.ubuntu.com/ubuntu/20260823T000000Z/|g' /etc/apt/sources.list.d/ubuntu.sources \
 && apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      build-essential bc bison flex libelf-dev libssl-dev \
 && rm -rf /var/lib/apt/lists/*
# Amazon's post-link FIPS HMAC step needs the CLI, not just libssl headers.
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssl \
 && rm -rf /var/lib/apt/lists/*
