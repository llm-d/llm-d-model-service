The files here are used for local development only.

To test `msvc-oci` on Kind, the cluster must be configured with the `ImageVolume` featuer enabled, otherwise the OCI volume will not get created in child deployments. Create the cluster with the following command.

```
kind create cluster --config samples/test/kind-config.yaml
```

Then, make install the requried CRDs. Refer to developer [docs](../../docs/developer.md).

Finally, apply the OCI example MSVC and baseconfig.

```
kubectl apply -f samples/test/baseconfig.yaml
kubectl apply -f samples/test/msvc-oci.yaml
kubectl get deploy busybox-decode -o yaml
```

and you should see the OCI image volume getting created. 