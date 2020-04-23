
FROM golang:1.14.2

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server .
EXPOSE 8080

ENTRYPOINT [ "./server" ]
CMD []
