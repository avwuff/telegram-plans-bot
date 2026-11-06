# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Build the app in its own stage, so the Go toolchain and build cache don't end up in the final image.
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY . .

# The vendor folder has the dependencies. CGO isn't needed, so the result doesn't depend on any system libraries.
RUN CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w" -o /out/main .

# The final image only has what the bot needs in order to run.
FROM alpine:3.24

# ca-certificates: needed for HTTPS connections to Telegram.
# tzdata: needed for time.LoadLocation. The bot panics on startup without it.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/main ./main
COPY html ./html
COPY dsn.txt salt.txt token.txt ./

# Expose port 8080 to the outside world
EXPOSE 8080

# Run the executable
CMD ["./main"]
