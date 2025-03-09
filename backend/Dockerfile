FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server ./cmd/server/main.go
EXPOSE 8080
CMD ["./server"]