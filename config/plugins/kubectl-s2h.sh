#!/bin/bash

# optional argument handling
if [[ "$1" == "version" ]]
then
    echo "1.0.1"
    exit 0
fi

# optional argument handling
if [[ "$1" == "config" ]]
then
    echo "$KUBECONFIG"
    exit 0
fi

if [[ "$1" == "activepromotion" ]] || [[ "$1" == "atp" ]]; then
    shift
    kubectl get activepromotion \
        -o=custom-columns=NAME:.metadata.name,STATE:.status.state \
        --sort-by="{.metadata.creationTimestamp}" \
        $@
    exit 0
fi

if [[ "$1" == "queue" ]] || [[ "$1" == "q" ]]; then
    shift
    kubectl get queue \
        -o=custom-columns=NS:.metadata.namespace,NAME:.metadata.name,VERSION:.spec.version,STATE:.status.state,ORDER:.spec.noOfOrder,RETRIES:.spec.noOfRetry,NOOFPROCESSES:.status.noOfProcessed,ENGINE:.status.deployEngine \
        --sort-by="{.spec.noOfOrder}" \
        $@
    exit 0
fi

if [[ "$1" == "queuehistory" ]] || [[ "$1" == "qh" ]]; then
    shift
    kubectl get queuehistory \
        -o=custom-columns=NS:.metadata.namespace,NAME:.spec.queue.spec.name,VERSION:.spec.queue.spec.version,DEPLOY:.spec.isDeploySuccess,TEST:.spec.isTestSuccess,NOOFPROCESSES:.spec.queue.status.noOfProcessed \
        --sort-by="{.spec.createdAt}" \
        $@
    exit 0
fi

if [[ "$1" == "env" ]]; then
    env|sort
    exit 0
fi

echo Welcome to Samsahai :)