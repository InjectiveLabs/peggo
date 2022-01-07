ARG IMG_TAG=latest

# Compile the peggo binary
FROM golang:1.17-alpine AS peggo-builder
WORKDIR /src/app/
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ENV PACKAGES make git libc-dev bash gcc linux-headers
RUN apk add --no-cache $PACKAGES
RUN make install

# Fetch umeed binary
FROM golang:1.17-alpine AS umeed-builder
ARG UMEE_VERSION=bez/gb-module-poc
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev
RUN apk add --no-cache $PACKAGES
WORKDIR /downloads/
RUN git clone https://github.com/umee-network/umee.git
RUN cd umee && git checkout ${UMEE_VERSION} && make build && cp ./build/umeed /usr/local/bin/

# Add to a distroless container
FROM gcr.io/distroless/cc:$IMG_TAG
ARG IMG_TAG
COPY --from=peggo-builder /go/bin/peggo /usr/local/bin/
COPY --from=umeed-builder /usr/local/bin/umeed /usr/local/bin/
EXPOSE 26656 26657 1317 9090
