FROM golang:1.20.4-bullseye

WORKDIR /app

COPY . .

RUN go install github.com/cosmtrek/air@latest

RUN go build -o app

EXPOSE 8080

CMD ["air", "-c", ".air.toml"]