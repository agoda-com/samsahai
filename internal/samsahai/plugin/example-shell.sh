#!/usr/bin/env bash

checkerName="example"
errRequestTimeout="request timeout"
errNoDesiredComponentVersion="no desired component version"
ErrImageVersionNotFound="image version not found"

get_name(){
    echo ${checkerName}
}

get_component() {
    name=$1
    if [[ "$name" == "Kubernetes" ]]; then
        echo "k8s"
        exit 0
    fi

    if [[ "$name" == "timeout" ]]; then
        sleep 10
        echo ${errRequestTimeout} >&2
        exit 1
    fi

    echo "$name"
    exit 0
}

get_version() {
    repository=$1
    name=$2
    pattern=$3

    if [[ "$name" == "not-found" ]]; then
        echo ${errNoDesiredComponentVersion} >&2
        exit 1
    fi

    if [[ "$name" == "timeout" ]]; then
        sleep 10
        echo ${errRequestTimeout} >&2
        exit 1
    fi

    if [[ "$name" == "fast-timeout" ]]; then
        echo ${errRequestTimeout} >&2
        exit 1
    fi

    if [[ "$name" == "example" ]]; then
        if [[ -z "$pattern" ]]; then
            echo "0.3.0"
            exit 0
        fi
        if [[ "0.2.0" =~ $pattern ]]; then
            echo "0.2.0"
            exit 0
        fi
        if [[ "0.1.1" =~ $pattern ]]; then
            echo "0.1.1"
            exit 0
        fi
        echo "0.3.0"
        exit 0
    fi
    echo ${errNoDesiredComponentVersion} >&2
    exit 1
}

ensure_version() {
    repository=$1
    name=$2
    version=$3

    if [[ "$name" == "example" ]]; then
        if [[ "0.1.1" =~ $version ]]; then
            echo "0.1.1"
            exit 0
        fi
    fi
    echo ${ErrImageVersionNotFound} >&2
    exit 1
}

subcommand=$1
case $subcommand in
    "get-name")
        shift
        get_name
        ;;
    "ensure-version")
        shift
        ensure_version $@
        ;;
    "get-version")
        shift
        get_version $@
        ;;
    "get-component")
        shift
        get_component $@
        ;;
    *)
        echo "invalid subcommand" >&2
        exit 1
        ;;
esac
