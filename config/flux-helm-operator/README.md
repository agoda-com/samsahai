### Helm Operator 
> by [Flux - Weaveworks](https://github.com/weaveworks/flux/)


#### Setup

1. Create ssh-key
    ```
    ssh-keygen -q -N "" -f ./identity 
    ```
1. Create secret
    ```
    kubectl create secret generic helm-operator-ssh --from-file=./identity
    ```
1. Configured `values.yaml` file or create new one
1. Install Helm Operator
    ```
    helm upgrade -i helm-operator .
    ```