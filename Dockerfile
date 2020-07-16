

# docker build -t sagecontinuum/sage-storage-api:latest .

FROM golang:1.14.2

WORKDIR /app

RUN go get -u gotest.tools/gotestsum


COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server .
EXPOSE 8080



ENTRYPOINT [ "./server" ]
CMD []
