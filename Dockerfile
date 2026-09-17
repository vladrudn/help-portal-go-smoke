FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS backend
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /help-portal-smoke .

FROM alpine:3.22
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=backend /help-portal-smoke /app/help-portal-smoke
COPY --from=frontend /app/web/dist /app/web/dist
COPY content /app/content
USER appuser
ENV PORT=8000
EXPOSE 8000
CMD ["/app/help-portal-smoke"]
