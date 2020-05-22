# Scorecard Operator

Run Scorecard tests via an operator running in the cluster

## Example

Create a bundle config map:
```
kubectl create configmap sample-bundle --from-file=bundle.tar.gz=testdata/bundle.tar.gz
```

Create a the CRDs and a sample test resource:
```
make install
kubectl apply -f ./config/samples/scorecard_v1alpha1_test.yaml
```

Run the operator:
```
make run
```

In another terminal, observe that the test object status has results
when the test pod completes:
```
kubectl get tests.scorecard.operatorframework.io test-sample -o yaml
```
