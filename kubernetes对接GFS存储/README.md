# Kubernetes 对接 GlusterFS 存储。
### 集群版本 1.18
### gfs operator https://github.com/gluster/gluster-kubernetes.git

### 先决条件
- 1、最低有三台可用的节点，如果只有两个 node，要确认 master 的污点是否取消
```
[root@kube01 ~]# kubectl get nodes
NAME     STATUS   ROLES         AGE     VERSION
kube01   Ready    master,node   5h27m   v1.18.6
kube02   Ready    node          5h26m   v1.18.6
kube03   Ready    node          119m    v1.18.6
```
- 2、三台node节点要有可使用的磁盘，这里可用硬盘为sdb
```
[root@kube01 ~]# lsblk
NAME              MAJ:MIN RM  SIZE RO TYPE MOUNTPOINT
sda                 8:0    0   20G  0 disk
├─sda1              8:1    0    1G  0 part /boot
└─sda2              8:2    0   19G  0 part
  └─centos-root     253:0  0   19G  0 lvm  /
sdb                 8:16   0   20G  0 disk
```
### 准备工作
- 1、给三台节点打上 glusterfs 的标签
```
kubectl label nodes --all storagenode=glusterfs
```
- 2、需要给三台节点安装 client 与配置内核模块
```
yum install -y glusterfs-client

sudo modprobe dm_thin_pool
sudo modprobe dm_mirror
sudo modprobe dm_snapshot
```
### 安装 GlusterFS
- 1、下载 gluster-kubernetes 代码
```
git clone https://github.com/gluster/gluster-kubernetes.git
```
- 2、配置 json 文件
```
cd gluster-kubernetes/deploy
cp topology.json.sample topology.json
[root@kube01 deploy]# cat topology.json
{
  "clusters": [
    {
      "nodes": [
        {
          "node": {
            "hostnames": {
              "manage": [
                "kube01"     #对应主机名
              ],
              "storage": [
                "192.168.245.129"  #对应IP
              ]
            },
            "zone": 1
          },
          "devices": [
            "/dev/sdb"  #对应数据卷
          ]
        },
        {
          "node": {
            "hostnames": {
              "manage": [
                "kube02"
              ],
              "storage": [
                "192.168.245.162"
              ]
            },
            "zone": 1
          },
          "devices": [
            "/dev/sdb"
          ]
        },
        {
          "node": {
            "hostnames": {
              "manage": [
                "kube03"
              ],
              "storage": [
                "192.168.245.163"
              ]
            },
            "zone": 1
          },
          "devices": [
            "/dev/sdb"
          ]
        }
      ]
    }
  ]
}
```
- 3、安装
```
### 因为gluster-kubernetes 项目 2019年7月后就不更新了，所以之前 kubernetes GV 为 extensions/v1beta1，
### api、daemonset 都需要修改 GV 为 apps/v1 修改后需要在 spec 部分添加 selector，因为 apps/v1 object 必须要带 selector
---
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: glusterfs
  ...
spec:
  selector:
    matchLabels:
      glusterfs: pod
      glusterfs-node: pod
  template:
  ...

### 创建 ns
kubectl create ns glusterfs

### 修改完成后 -g 是部署
./gk-deploy -g --admin-key adminkey --user-key userkey -y -n glusterfs

### 如果执行失败需要在三台节点执行一下步骤
#### 查看是否创建 vg 了，如果创建删除 vgremove vg_ccc7b4a115742853db1d7ca0d6ad65e7
[root@kube01 deploy]# vgs
  VG                                  #PV #LV #SN Attr   VSize   VFree
  centos                                1   1   0 wz--n- <19.00g     0
  vg_ccc7b4a115742853db1d7ca0d6ad65e7   1   4   0 wz--n-  19.87g 12.80g

#### 删除文件夹
dmsetup ls
dmsetup remove_all
rm -rf /var/log/glusterfs/
rm -rf /var/lib/heketi
rm -rf /var/lib/glusterd/
rm -rf /etc/glusterfs/
dd if=/dev/zero of=/dev/sdb bs=512k count=1   # 这里的/dev/sdb是要写你配置的硬盘路径

### 部署节点执行卸载任务
./gk-deploy --abort --admin-key adminkey --user-key userkey -y -n glusterfs
```
### 测试
- 1、可以直接进 pod 测试
```
[root@kube01 deploy]# kubectl get pod -n glusterfs
NAME                     READY   STATUS    RESTARTS   AGE
glusterfs-kdkfg          1/1     Running   0          98m
glusterfs-vvhs4          1/1     Running   1          98m
glusterfs-xtjrk          1/1     Running   0          98m
heketi-b547fdd54-6gb5q   1/1     Running   0          96m
[root@kube01 ~]# kubectl exec -ti glusterfs-kdkfg -n glusterfs bash
[root@kube03 /]# gluster volume list

[root@kube01 ~]# kubectl exec -ti heketi-b547fdd54-6gb5q -n glusterfs bash
[root@kube03 /]# heketi-cli --user admin --secret adminkey cluster list
Clusters:
Id:0ee0286497d04d1e99442b89354bd9bc [file][block]
```
### 对接 sc
- 1、创建 sc yaml 文件
```
### 需要先创建 secret
cat storageclass-dev-glusterfs.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: heketi-secret
  namespace: glusterfs
data:
  # base64 encoded password. E.g.: echo -n "adminkey" | base64
  key: YWRtaW5rZXk=
type: kubernetes.io/glusterfs

---
apiVersion: storage.k8s.io/v1beta1
kind: StorageClass
metadata:
  name: glusterfs
provisioner: kubernetes.io/glusterfs
parameters:
  resturl: "http://10.8.4.91:42951"     ### 地址是 kubectl get svc heketi -n glusterfs -o go-template='{{.spec.clusterIP}}':8080 获取
  clusterid: "0ee0286497d04d1e99442b89354bd9bc" ### clusterid 是 heketi-cli --user admin --secret adminkey cluster list 获取
  restauthenabled: "true"
  restuser: "admin"                 
  secretNamespace: "glusterfs"     ### secretNamespace 是 secret 的 ns 
  secretName: "heketi-secret"      ### secretNamespace 是 secret 的 name
  #restuserkey: "adminkey"
  gidMin: "40000"
  gidMax: "50000"
  volumetype: "none"
```
- 2、创建 sc
```
kubectl apply -f glusterfs-storageclass.yaml
```
- 3、创建 pvc 测试
```
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: task-pv-claim
spec:
  storageClassName: glusterfs
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 3Gi
```
- 4、查看 pvc
```
[root@kube01 ~]# kubectl get sc
NAME                  PROVISIONER               RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
glusterfs             kubernetes.io/glusterfs   Delete          Immediate           false                  59m

[root@kube01 ~]# kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS      CLAIM                   STORAGECLASS   REASON   AGE
pvc-c27b0cab-60df-4627-8928-24e0007cbffd   3Gi        RWO            Delete           Bound       default/task-pv-claim   glusterfs               57m

[root@kube01 ~]# kubectl get pvc
NAME            STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
task-pv-claim   Bound    pvc-c27b0cab-60df-4627-8928-24e0007cbffd   3Gi        RWO            glusterfs      55m
```