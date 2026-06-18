FROM golang:1.21-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o app main.go

FROM alpine
WORKDIR /app
COPY --from=build /app/app .
EXPOSE 8080
CMD ["./app"]