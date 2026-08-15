# ---- stage 1: frontend build ----
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- stage 2: backend build ----
FROM golang:1.25-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN rm -rf internal/web/dist
COPY --from=frontend-build /app/frontend/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o /leaflag ./cmd/leaflag

# ---- stage 3: final ----
FROM gcr.io/distroless/static-debian12
COPY --from=backend-build /leaflag /leaflag
EXPOSE 8110
ENTRYPOINT ["/leaflag"]
