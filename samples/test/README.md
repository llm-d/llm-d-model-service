The files here are used for local development only.

To test `msvc-oci` on Kind, the cluster must be configured with the `ImageVolume` feature gate enabled, otherwise the OCI volume will not get created in child deployments. Create the cluster with the following command.

```
kind create cluster --image="kindest/node:v1.32.0@sha256:c48c62eac5da28cdadcf560d1d8616cfa6783b58f0d94cf63ad1bf49600cb027" --config samples/test/kind-config.yaml
```

Then, make install the requried CRDs. Refer to developer [docs](../../docs/developer.md).

Finally, apply the OCI example MSVC and baseconfig.

```
kubectl apply -f samples/test/baseconfig.yaml
kubectl apply -f samples/test/msvc-oci.yaml
kubectl get deploy busybox-decode -o yaml
```

and you should see the OCI image volume getting created. 