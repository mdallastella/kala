# syntax=docker/dockerfile:1
FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /kala ./cmd/kala

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /kala /kala
ENV KALA_ADDR=:8080 KALA_DB_PATH=/data/kala.db
VOLUME /data
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/kala"]
