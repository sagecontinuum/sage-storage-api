FROM golang:1.13.8
LABEL maintainer="iperezx <i3perez@sdsc.edu>"
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server .
EXPOSE 8080
ENTRYPOINT [ "./server" ]
CMD []