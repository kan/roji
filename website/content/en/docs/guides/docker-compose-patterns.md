---
title: "Docker Compose Patterns"
description: "Common Docker Compose configurations with roji."
weight: 6
date: 2024-01-01T00:00:00+00:00
lastmod: 2026-03-04T00:00:00+00:00
draft: false
toc: true
---

Here are common Docker Compose patterns that work well with roji.

## Frontend + API + Database

A typical full-stack application with a frontend, API, and database. Only the frontend and API need to be on the roji network — the database stays internal.

```yaml
services:
  frontend:
    image: node:alpine
    working_dir: /app
    command: ["npm", "run", "dev"]
    volumes:
      - ./frontend:/app
    expose:
      - "3000"
    networks:
      - roji
      - internal

  api:
    image: node:alpine
    working_dir: /app
    command: ["npm", "start"]
    volumes:
      - ./api:/app
    expose:
      - "4000"
    environment:
      - DATABASE_URL=postgres://postgres:password@db:5432/myapp
    networks:
      - roji
      - internal

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: password
      POSTGRES_DB: myapp
    volumes:
      - pgdata:/var/lib/postgresql/data
    networks:
      - internal    # Not on roji — no external access needed

volumes:
  pgdata:

networks:
  roji:
    external: true
  internal:
    driver: bridge
```

Access:
- `https://frontend.dev.localhost` — React/Vue dev server
- `https://api.dev.localhost` — API server

## Path-based Routing (Single Domain)

Serve both frontend and API under the same hostname:

```yaml
services:
  frontend:
    image: nginx:alpine
    labels:
      - "roji.host=myapp.dev.localhost"
    networks:
      - roji

  api:
    image: my-api
    labels:
      - "roji.host=myapp.dev.localhost"
      - "roji.path=/api"
    networks:
      - roji

networks:
  roji:
    external: true
```

Access:
- `https://myapp.dev.localhost` — frontend
- `https://myapp.dev.localhost/api/*` — API

## Microservices

Multiple services, each with its own subdomain:

```yaml
services:
  auth:
    image: auth-service
    expose: ["8080"]
    networks: [roji]

  users:
    image: users-service
    expose: ["8080"]
    networks: [roji]

  orders:
    image: orders-service
    expose: ["8080"]
    networks: [roji]

  gateway:
    image: api-gateway
    expose: ["8080"]
    labels:
      - "roji.host=api.dev.localhost"
    networks: [roji]

networks:
  roji:
    external: true
```

Access:
- `https://auth.dev.localhost`
- `https://users.dev.localhost`
- `https://orders.dev.localhost`
- `https://api.dev.localhost` — API gateway

## WordPress

```yaml
services:
  wordpress:
    image: wordpress:latest
    expose:
      - "80"
    labels:
      - "roji.host=blog.dev.localhost"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wordpress
      WORDPRESS_DB_PASSWORD: wordpress
      WORDPRESS_DB_NAME: wordpress
    volumes:
      - wp_data:/var/www/html
    networks:
      - roji
      - internal

  db:
    image: mysql:8
    environment:
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wordpress
      MYSQL_PASSWORD: wordpress
      MYSQL_ROOT_PASSWORD: rootpassword
    volumes:
      - db_data:/var/lib/mysql
    networks:
      - internal

volumes:
  wp_data:
  db_data:

networks:
  roji:
    external: true
  internal:
    driver: bridge
```

Access: `https://blog.dev.localhost`

## WebSocket Application

WebSocket connections are proxied automatically when the `Upgrade: websocket` header is detected:

```yaml
services:
  chat:
    image: my-chat-app
    expose:
      - "8080"
    networks:
      - roji

networks:
  roji:
    external: true
```

Connect via `wss://chat.dev.localhost` — no additional configuration needed.

## gRPC Service

gRPC services are proxied over HTTP/2 when `Content-Type: application/grpc` is detected:

```yaml
services:
  grpc-api:
    image: my-grpc-service
    expose:
      - "50051"
    labels:
      - "roji.port=50051"
    networks:
      - roji

networks:
  roji:
    external: true
```

Connect to `grpc-api.dev.localhost:443` using your gRPC client.
