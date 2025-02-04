# Cloud Provider Operator

이 프로젝트는 Kubernetes 클러스터에서 OpenStack 인스턴스를 관리하기 위한 오퍼레이터입니다. 이 오퍼레이터는 사용자 정의 리소스(CR)를 통해 OpenStack 인스턴스의 생성, 업데이트, 삭제를 자동화합니다.

## 개요

`Instance` CRD는 OpenStack 인스턴스의 스펙을 정의하며, 오퍼레이터는 이 스펙을 기반으로 인스턴스를 관리합니다. 이 오퍼레이터는 Pulumi를 사용하여 OpenStack 리소스를 프로비저닝합니다.

## CRD 정의

`Instance` CRD는 다음과 같은 필드를 포함합니다:

- **Spec**
  - `FlavorName`: 인스턴스의 플레이버 이름
  - `ImageName`: 인스턴스의 이미지 이름
  - `NetworkUUID`: 인스턴스가 연결될 네트워크의 UUID

- **Status**
  - 현재는 정의된 필드가 없으며, 향후 인스턴스의 상태를 반영할 수 있습니다.

## 설치

1. Kubernetes 클러스터에 CRD를 적용합니다.
   ```bash
   kubectl apply -f config/crd/bases/infrastructure.cloudprovider.io_instances.yaml
   ```

2. 오퍼레이터를 배포합니다.
   ```bash
   kubectl apply -f config/deploy/operator.yaml
   ```

## 사용법

1. `Instance` 리소스를 생성하여 OpenStack 인스턴스를 프로비저닝합니다.
   ```yaml
   apiVersion: infrastructure.cloudprovider.io/v1alpha1
   kind: Instance
   metadata:
     name: example-instance
     namespace: default
   spec:
     flavorName: "m1.small"
     imageName: "ubuntu-20.04"
     networkUUID: "123e4567-e89b-12d3-a456-426614174000"
   ```

2. 생성한 리소스를 적용합니다.
   ```bash
   kubectl apply -f instance.yaml
   ```

3. 인스턴스의 상태를 확인합니다.
   ```bash
   kubectl get instances
   ```

## 개발

CRD 정의를 수정한 후에는 코드를 재생성해야 합니다.
   ```bash
   make generate
   ```