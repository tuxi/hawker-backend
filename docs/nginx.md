## 在docker compose中部署 nginx conf
在 docker-compose.yaml 中增加一个 Nginx 服务，实现“全家桶”式部署
- 1.编写 nginx.conf
  * 在项目根目录下创建一个 nginx 文件夹，并新建 `hawker.conf` 文件：
```nginx
server {
listen 80;
server_name 43.156.94.29;

    # 1. 静态音频资源
    location /static/ {
        alias /app/static/;
        add_header Access-Control-Allow-Origin *;
    }

    # 2. 核心：处理所有 API 和 WebSocket 请求
    location /api/v1/ {
        proxy_pass http://hawker-backend:12188; # 指向 Go 后端容器

        # 基础 Proxy 头信息
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        # 针对 WS 的超时优化（建议设为 1 小时，由 Go 后端通过心跳控制断开）
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;

        # 禁用缓存，确保实时性
        proxy_buffering off;
    }
}
```

- 2.更新 `docker-compose.yaml`
> 将 Nginx 加入到编排中，让它挡在最前面：
```
services:
  # Nginx 服务
  nginx:
    image: nginx:alpine
    container_name: hawker-nginx
    ports:
      - "80:80"   # 映射 HTTP 端口
      # - "443:443" # 如果有 SSL，开启此端口
    volumes:
      - ./nginx/hawker.conf:/etc/nginx/conf.d/default.conf:ro
      - ./static:/app/static:ro # 🌟 关键：让 Nginx 也能读取音频文件目录
      - ./logs/nginx:/var/log/nginx
    depends_on:
      - hawker-app
    networks:
      - hawker-net
  #后端服务
  hawker-app:
    # 🌟 关键：告诉 compose 从当前目录构建镜像
    build: .
    image: hawker-backend:latest
    container_name: hawker-backend
    # ports:
    #   - "12188:12188" # 屏蔽外部直接访问，只通过 Nginx 访问
    volumes:
      - ./conf:/app/hawker-backend/conf
      - ./logs:/app/hawker-backend/logs
      - ./static:/app/hawker-backend/static
    restart: always
    networks:
      - hawker-net
    environment:
      # 如果是同机部署，尽量换成内网 IP
      DB_HOST: "43.156.94.29"
      DB_PORT: 5432
      DB_USER: postgres
      DB_PASSWORD: "root123" # 建议用引号包裹，防止特殊字符解析错误
      DB_NAME: hawker_db
      TEST: ""

networks:
  hawker-net:
    driver: bridge
```
- 3.部署和测试
```
docker-compose up -d --build
```