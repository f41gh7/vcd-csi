# vCloud director CSI controller


CSI for vCloud director, it uses independent disks for volume and attach/detach it.


## quick start

 - ensure that kubectl get nodes and vm name at VDC is the same
 - label node for each vdc
 ```bash 
kubectl label node nod1  topology.kubernetes.io/vcd=some-vdc
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
 - edit settings for controller addd correct VDC name - VCSI_VDCS: some-vdc
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
                  - key: "topology.kubernetes.io/vcd"
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

- parametr for storage profile must be defirened at storage class creation


## limitations

- tested at ubuntu 16 with Paravirtual SCSI only
- computer name at vcd vapp MUST match k8s node name
- permissions - csi wants admin permissions at vdc
- for multiple vdcs you have to provide it as list ```VCSI_VDCNAMES=vcd-1,vcd-2```
  and tag node + storageclass
- zone-aware - VDC name
  so common practice to run different daemonsets
  for each VDC place them with node affinity

## requirements

- Docker mount propagtion = shared
- kubernetes 14+


# known problems

 Its hard to link attacheted to VM disk, currently there is filter:
 - list all disks and match it by disk size
 - list matched by size disk and check unit number for PCI: bus, it must contain unit number
   from vm setting
   
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
