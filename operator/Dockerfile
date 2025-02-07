# Build the manager binary
FROM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Pulumi 설치
RUN curl -fsSL https://get.pulumi.com | sh
ENV PATH=$PATH:/root/.pulumi/bin

# Pulumi 버전 확인
RUN pulumi version

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Build with CGO_ENABLED=1
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Pulumi 바이너리에 실행 권한 추가 (builder 단계에서 처리)
RUN chmod +x /root/.pulumi/bin/pulumi

# Use Alpine as minimal base image
FROM alpine:latest
WORKDIR /

# 필수 패키지 설치 (glibc, ca-certificates)
RUN apk --no-cache add ca-certificates libc6-compat

# 실행 가능한 Pulumi 바이너리를 복사
COPY --from=builder /root/.pulumi/bin/pulumi /usr/local/bin/pulumi
COPY --from=builder /workspace/manager /manager

# Pulumi 로컬 백엔드 및 암호화 비활성화 설정
ENV PULUMI_CONFIG_PASSPHRASE=""
ENV PULUMI_BACKEND_URL="file:///workspace/pulumi"
ENV PULUMI_SKIP_TLS_VERIFY=true
ENV PULUMI_HOME=/workspace/pulumi

# 디렉토리 권한 설정
RUN mkdir -p /workspace/pulumi && chown -R 65532:65532 /workspace/pulumi

# 추가: $USER 환경 변수 설정 (숫자 ID 사용)
ENV USER=65532

# 비-루트 사용자 실행 (숫자 ID 사용)
USER 65532:65532

ENTRYPOINT ["/manager"]

