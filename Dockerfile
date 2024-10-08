#install packages for build layer
FROM golang:1.22-alpine as builder

ADD https://github.com/CosmWasm/wasmvm/releases/download/v2.1.2/libwasmvm_muslc.aarch64.a /lib/libwasmvm_muslc.aarch64.a
ADD https://github.com/CosmWasm/wasmvm/releases/download/v2.1.2/libwasmvm_muslc.x86_64.a /lib/libwasmvm_muslc.x86_64.a

RUN apk add --no-cache git gcc make perl jq libc-dev linux-headers libgcc

#Set architecture
RUN apk --print-arch > ./architecture
RUN cp /lib/libwasmvm_muslc.$(cat ./architecture).a /lib/libwasmvm_muslc.a
RUN rm ./architecture

#build binary
WORKDIR /src
COPY . .
RUN go mod download

#install binary
RUN DOCKER=true make install

#build main container
#FROM alpine:latest
RUN apk add --update --no-cache ca-certificates curl libgcc
#COPY --from=builder /go/bin/* /usr/local/bin/

#configure container
VOLUME /apps/data
WORKDIR /root/.injectived/peggo

#default command
CMD peggo orchestrator
