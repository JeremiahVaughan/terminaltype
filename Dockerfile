FROM public.ecr.aws/docker/library/golang:1.23-alpine3.21 as builder
RUN mkdir -p /workspace
WORKDIR /workspace

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify

COPY . .
# Use the ARG for GOARCH here

RUN CGO_ENABLED=0 GOOS=linux go build -o ./app .

FROM public.ecr.aws/docker/library/alpine:latest
COPY --from=builder /workspace/app /app
ENTRYPOINT ["/app"]
