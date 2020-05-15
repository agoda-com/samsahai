#!/usr/bin/env bash

set -ex

BASEDIR=$(dirname "$0")
K8S_VERSION=${K8S_VERSION:-v1.14.4}
KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION:-3.0.1}
HELM_VERSION=${HELM_VERSION:-v2.13.1}
MINIKUBE_VERSION=${MINIKUBE_VERSION:-v1.2.0}
INSTALL_DIR=${INSTALL_DIR:-/usr/local/bin/}

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


echo Install kubectl ${K8S_VERSION}

curl -Lo kubectl https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/${OS}/${ARCH}/kubectl \
  && chmod +x kubectl \
  && sudo mv kubectl ${INSTALL_DIR}
mkdir -p ${HOME}/.kube
touch ${HOME}/.kube/config


echo Install kustomize ${KUSTOMIZE_VERSION}
curl -Lo kustomize https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_${OS}_${ARCH} \
  && chmod +x kustomize \
  && sudo mv kustomize ${INSTALL_DIR}


echo Install helm ${HELM_VERSION}

HELM_DIST="helm-${HELM_VERSION}-${OS}-${ARCH}.tar.gz"
curl -Lo ${HELM_DIST} https://storage.googleapis.com/kubernetes-helm/${HELM_DIST} \
  && tar -xf ${HELM_DIST} \
  && chmod +x ${OS}-${ARCH}/helm \
  && sudo mv ${OS}-${ARCH}/helm ${INSTALL_DIR} \
  && rm -f ${HELM_DIST} \
  && rm -rf ${OS}-${ARCH}


echo Install minikube ${MINIKUBE_VERSION}

curl -Lo minikube https://github.com/kubernetes/minikube/releases/download/${MINIKUBE_VERSION}/minikube-${OS}-${ARCH} \
  && chmod +x minikube \
  && sudo mv minikube ${INSTALL_DIR}


# start minikube
if [[ "$OS" = "linux" ]]; then
  echo Start minikube on linux
  sudo -E minikube start --vm-driver=none --cpus 2 --memory 4096 --kubernetes-version=${K8S_VERSION}
else
  echo Start minikube on mac osx
  minikube start --vm-driver=hyperkit --cpus=2 --memory 4096
fi


echo Checking minikube is running

until ${INSTALL_DIR}/kubectl get pod > /dev/null 2>&1
do
  sleep 5
done

#alias kubectl=${INSTALL_DIR}kubectl
#alias helm=${INSTALL_DIR}helm
#alias kustomize=${INSTALL_DIR}kustomize

kubectl apply -f $BASEDIR/helm-rbac.yaml
helm init --service-account tiller

CONFIGDIR=$BASEDIR/../config

# install crds
${INSTALL_DIR}/kubectl apply -f $CONFIGDIR/crds

# wait for pod to ready
echo Wait tiller and helm-operator to be ready

kubectl -n kube-system wait Pods -l app=helm --for=condition=Ready --timeout=5m
kubectl -n kube-system wait Pods -l app=helm-operator --for=condition=Ready --timeout=5m
