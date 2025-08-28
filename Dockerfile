FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ngobrolyuk main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/ngobrolyuk .
EXPOSE 8080
CMD ["./ngobrolyuk"]