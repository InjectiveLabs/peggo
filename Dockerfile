ARG IMG_TAG=latest

# Fetch base packages
FROM golang:1.18-alpine AS base-builder
ENV PACKAGES make git libc-dev gcc linux-headers
RUN apk add --no-cache $PACKAGES

# Compile the peggo binary
FROM base-builder AS peggo-builder
WORKDIR /src/app/
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN apk add --no-cache $PACKAGES
RUN make install

# Fetch umeed binary
FROM base-builder AS umeed-builder
ARG UMEE_VERSION=v2.0.0
ENV PACKAGES curl eudev-dev
RUN apk add --no-cache $PACKAGES
WORKDIR /downloads/
RUN git clone https://github.com/umee-network/umee.git
RUN cd umee && git checkout ${UMEE_VERSION} && CGO_ENABLED=0 make build && cp ./build/umeed /usr/local/bin/

# Add to a distroless container
FROM gcr.io/distroless/cc:$IMG_TAG
ARG IMG_TAG
COPY --from=peggo-builder /go/bin/peggo /usr/local/bin/
COPY --from=umeed-builder /usr/local/bin/umeed /usr/local/bin/
EXPOSE 26656 26657 1317 9090
