#!/bin/bash

KUBECTL_S2H_PATH=$(which kubectl-s2h)

if [ -z "$KUBECTL_S2H_PATH" ]; then
    KUBECTL_DIR=$(dirname $(which kubectl))
    cp kubectl-s2h.sh $KUBECTL_DIR/kubectl-s2h
else
    cp kubectl-s2h.sh $KUBECTL_S2H_PATH
fi