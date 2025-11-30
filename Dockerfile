# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.22 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://proxy.golang.org,direct
COPY . .

# Build for the target platform (supports both amd64 and arm64)
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /out/node-life-support ./main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/node-life-support /node-life-support
USER nonroot
ENTRYPOINT ["/node-life-support"]
