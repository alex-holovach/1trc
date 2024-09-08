FROM golang:1.23 AS builder

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /backend

FROM golang:1.23

COPY --from=builder /backend /app/backend

WORKDIR /app

EXPOSE 8080

CMD ["/app/backend"]

