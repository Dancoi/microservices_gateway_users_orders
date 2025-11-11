FROM golang:1.21-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/api_gateway ./...

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /usr/local/bin/api_gateway /usr/local/bin/api_gateway
EXPOSE 8000
ENV PORT=8000
ENTRYPOINT ["/usr/local/bin/api_gateway"]