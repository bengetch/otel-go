FROM golang:1.22.1 as builder

WORKDIR /app

# copy requirements file, download requirements
COPY go.mod go.sum ./
RUN go mod download
# copy rest of dependencies
COPY . .

# build app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o service .

FROM alpine:latest

WORKDIR /root/

# copy binary file from the previous stage
COPY --from=builder /app/service .

# run executable
CMD ["./service"]