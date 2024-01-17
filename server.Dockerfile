FROM golang:1.21 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -v -a -o /server cmd/server/server.go

FROM alpine
WORKDIR /app
COPY --from=builder /server /app/server
COPY quotes.json /app/quotes.json

EXPOSE 8080

ENTRYPOINT ["/app/server"]