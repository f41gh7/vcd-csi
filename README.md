# vCloud director CSI controller


CSI for vCloud director, it uses independent disks for volume and attach/detach it.

inspired by https://github.com/flant/yandex-csi-driver

## quick start

 - ensure that kubectl get nodes and vm name at VDC is the same
 - label node for each vdc
 ```bash 
kubectl label node nod1  failure-domain.beta.kubernetes.io/zone=some-vdc
```
 - create secret for access cloud - you can find example at deploy/secret.yaml
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vcd-csi-credentials
  namespace: kube-system
stringData:
  USER: some-user
  PASSWOR: some-password
  ORG: Org-Name
  VDCS: some-vdc
  HREF: https://vcd.cloud/api

```
 - create rbac
   ```bash
   kubectl apply -f deploy/rbac.yaml
   ```
 - create controller
 ```bash
  kubectl apply -f deploy/controller.yaml
 ```
 - create storage class
 ```yaml
---
apiVersion: storage.k8s.io/v1beta1
kind: StorageClass
metadata:
  name: base-hdd
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: vcd.csi.fght.net
parameters:
  vcd: some-vdc
  storageProfile: hdd
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```
- edit node daemon set deploy/ds.yaml
```bash
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: "topology.kubernetes.io/zone"
                    operator: In
                    values:
                      - "some-vdc"
```
and  for csi-node
```yaml
            - name: VCSI_NODEVDC
              value: some-vdc

```


## settings

- csi controller uses env vars for config, you can find it at vars.MD

- parameter for storage profile must be definened at storage class creation


## limitations

- for disk resize you must edit pvc, then delete pod from node, disk will be detached and resized
- vm disk controllers cannot containt attached disk for units 8-15, it reserved for CSI
- so maximum attached disk per vm is 8
- tested at ubuntu 16 with Paravirtual SCSI only
- computer name at vdc vapp MUST match k8s node name
- permissions - csi wants admin permissions at vdc
- for multiple vdcs you have to provide it as list ```VCSI_VDCS=vcd-1,vcd-2``` at secret
  and tag node + vdc name topology tag
- zone-aware - VDC name
  so common practice to run different daemonsets
  for each VDC with node affinity

## requirements

- Docker mount propagtion = shared
- kubernetes 14+, --allow-privileged
- feature gate ```KubeletPluginsWatcher=true,CSINodeInfo=true,CSIDriverRegistry=true```


# known problems

 Its hard to link attacheted to VM disk, currently there is filter:
 - list matched by unit number for PCI: bus, it must contain unit number
   from vm setting
   so unit numbers from 8 to 15 reserved for all controllers.
   
 But there can be collisions, so whats the solution? use different vm for disk or change disk size..
 
 Unfortunately Bus number at VM settings isnt reliable. Looking for best solution.   

## developing
- you need go1.13+
- vcd connections for integration tests

## todo

- leader election and clustering for controller
- volume expand (seems like we need distributed lock)
- more test

## testing

make test

https://github.com/kubernetes-csi/csi-test/blob/master/pkg/sanity/README.md
