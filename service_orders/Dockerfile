FROM golang:1.21-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usr/local/bin/service_orders ./... || true

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /usr/local/bin/service_orders /usr/local/bin/service_orders
EXPOSE 8000
ENV PORT=8000
ENTRYPOINT ["/usr/local/bin/service_orders"]