FROM golang:1.20.3-bullseye as builder
WORKDIR /src
ADD . .
RUN go build ./cmd/ramdisk

FROM registry.k8s.io/build-image/debian-base:bullseye-v1.4.3
RUN clean-install mount
COPY --from=builder /src/ramdisk /ramdisk
ENTRYPOINT ["/ramdisk"]
