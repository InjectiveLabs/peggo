#install packages for build layer
FROM golang:1.18.3-alpine as builder
RUN apk add --no-cache git gcc make perl jq libc-dev linux-headers

ADD https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.aarch64.a /lib/libwasmvm_muslc.aarch64.a
ADD https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.x86_64.a /lib/libwasmvm_muslc.x86_64.a
RUN sha256sum /lib/libwasmvm_muslc.aarch64.a | grep 7d2239e9f25e96d0d4daba982ce92367aacf0cbd95d2facb8442268f2b1cc1fc
RUN sha256sum /lib/libwasmvm_muslc.x86_64.a | grep f6282df732a13dec836cda1f399dd874b1e3163504dbd9607c6af915b2740479

#Set architecture
RUN apk --print-arch > ./architecture
RUN cp /lib/libwasmvm_muslc.$(cat ./architecture).a /lib/libwasmvm_muslc.a
RUN rm ./architecture


#build binary
WORKDIR /src
COPY . .
RUN go mod download

#install binary
RUN LEDGER_ENABLED=false BUILD_TAGS=muslc LINK_STATICALLY=true make install

#build main container
FROM alpine:latest
RUN apk add --update --no-cache ca-certificates
RUN apk add curl
COPY --from=builder /go/bin/* /usr/local/bin/

#configure container
VOLUME /apps/data
WORKDIR /apps/data

#default command
CMD cd /root/.injectived/peggo/ && peggo orchestrator
