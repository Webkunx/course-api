FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /course-api .

FROM alpine:3.21

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app

COPY --from=builder /course-api /app/course-api
COPY --from=builder /app/db/migrations /app/db/migrations

USER app
EXPOSE 3000

ENTRYPOINT ["/app/course-api"]
