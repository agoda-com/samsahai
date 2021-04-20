#!/bin/sh

checkerName="cspider"

get_name(){
    echo ${checkerName}
}

subcommand=$1
case $subcommand in
    "get-name")
        shift
        get_name
        ;;
    *)
        cinterny s2h checker agoda-cspider $@
        ;;
esac
