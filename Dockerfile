# ---- stage 1: build (the kitchen) --------
FROM golang:1.27.1 AS build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/order-api ./cmd/order-api

# ------------- Stage 2: Runtime (the delivery box) ---------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/order-api /order-api
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/order-api"]      