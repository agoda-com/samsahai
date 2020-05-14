#!/usr/bin/env bash

set -eux

BASEDIR=$(dirname "$0")
K8S_VERSION=${K8S_VERSION:-v1.14.4}
KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-3.1.0}
HELM_VERSION=${HELM_VERSION:-v2.13.1}
MINIKUBE_VERSION=${MINIKUBE_VERSION:-v1.2.0}
K3D_VERSION=${K3D_VERSION:-v1.3.1}
INSTALL_DIR=${INSTALL_DIR:-/usr/local/bin/}

kubectl="${INSTALL_DIR}/kubectl"
helm="${INSTALL_DIR}/helm"
kustomize="${INSTALL_DIR}/kustomize"
k3d="${INSTALL_DIR}/k3d"

# initArch discovers the architecture for this system.
initArch() {
  ARCH=$(uname -m)
  case $ARCH in
    armv5*) ARCH="armv5";;
    armv6*) ARCH="armv6";;
    armv7*) ARCH="arm";;
    aarch64) ARCH="arm64";;
    x86) ARCH="386";;
    x86_64) ARCH="amd64";;
    i686) ARCH="386";;
    i386) ARCH="386";;
  esac
}


# initOS discovers the operating system for this system.
initOS() {
  OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

  case "$OS" in
    # Minimalist GNU for Windows
    mingw*) OS='windows';;
  esac
}


initArch
initOS


if $kubectl version --client --short | grep -v ${K8S_VERSION}; then
  echo Install kubectl ${K8S_VERSION}

  curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${OS}/${ARCH}/kubectl \
    && chmod +x kubectl \
    && sudo mv kubectl ${INSTALL_DIR}
  mkdir -p ${HOME}/.kube
  touch ${HOME}/.kube/config
fi

if $kustomize version | grep -v ${KUSTOMIZE_VERSION}; then
  echo Install kustomize ${KUSTOMIZE_VERSION}

  curl -Lo kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_${OS}_${ARCH} \
    && chmod +x kustomize \
    && sudo mv kustomize ${INSTALL_DIR}
fi

if $helm version --client --short | grep -v ${HELM_VERSION}; then
  echo Install helm ${HELM_VERSION}

  HELM_DIST="helm-${HELM_VERSION}-${OS}-${ARCH}.tar.gz"
  curl -Lo ${HELM_DIST} https://storage.googleapis.com/kubernetes-helm/${HELM_DIST} \
    && tar -xf ${HELM_DIST} \
    && chmod +x ${OS}-${ARCH}/helm \
    && sudo mv ${OS}-${ARCH}/helm ${INSTALL_DIR} \
    && rm -f ${HELM_DIST} \
    && rm -rf ${OS}-${ARCH}
fi

if $k3d --version | grep -v ${K3D_VERSION}; then
  echo Install k3d ${K3D_VERSION}

  curl -Lo k3d https://github.com/rancher/k3d/releases/download/${K3D_VERSION}/k3d-${OS}-${ARCH} \
    && chmod +x k3d \
    && sudo mv k3d ${INSTALL_DIR}
fi

#$kubectl get pod
K3D_CLUSTER_NAME=k3s-default

if $k3d ls | grep running | grep -v "${K3D_CLUSTER_NAME}"; then
  $k3d create
elif $k3d ls 2>&1 | grep -i "no clusters found"; then
  $k3d create
fi

$k3d get-kubeconfig

export KUBECONFIG=$($k3d get-kubeconfig)

#$kubectl version
#$helm version

echo Checking minikube is running

until $kubectl get pod > /dev/null 2>&1
do
  sleep 5
done
g p
$kubectl apply -f $BASEDIR/helm-rbac.yaml
$helm init --service-account tiller

CONFIGDIR=$BASEDIR/../config

# install crds
$kubectl apply -f $CONFIGDIR/crds

# wait for pod to ready
echo Wait tiller and helm-operator to be ready

$kubectl -n kube-system wait Pods -l app=helm --for=condition=Ready --timeout=5m
$kubectl -n kube-system wait Pods -l app=helm-operator --for=condition=Ready --timeout=5m

echo "run export KUBECONFIG=\$(k3d get-kubeconfig)"
