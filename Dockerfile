# --- Builder Stage ---
# 使用 1.25-alpine 会自动匹配到当前的 1.25.4 或更高补丁版本
FROM golang:1.25-alpine AS builder
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOPROXY=https://goproxy.io,direct

RUN apk add --no-cache git make
WORKDIR /code
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 直接编译，不再运行 swag init
RUN make linux

# --- Final Stage ---
FROM alpine:latest
RUN apk --no-cache add tzdata ca-certificates libc6-compat
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app/hawker-backend
COPY --from=builder /code/dist/linux_amd64/hawker-backend .

EXPOSE 12188
VOLUME ["/app/hawker-backend/conf", "/app/hawker-backend/logs", "/app/hawker-backend/static"]

ENTRYPOINT ["./hawker-backend"]