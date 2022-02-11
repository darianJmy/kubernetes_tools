# Kubeadm 安装 Kubernetes 1.20.1 版本集群后，默认是 IPv4 单栈模式，通过修改配置文件实现 IPv4/IPv6 双栈模式。

- 先决条件
```
为了使用 IPv4/IPv6 双栈的 Kubernetes 集群，需要满足以下先决条件：

Kubernetes 1.20 版本或更高版本，有关更早 Kubernetes 版本的使用双栈服务的信息， 请参考对应版本的 Kubernetes 文档。
提供商支持双协议栈网络（云提供商或其他提供商必须能够为 Kubernetes 节点提供可路由的 IPv4/IPv6 网络接口）。
支持双协议栈的网络插件（如 Kubenet 或 Calico）。

默认从 1.21 版本开始，IPv4/IPv6 双协议栈默认是被启用的，低于此版本的要开启双栈特性 IPv6DualStack。
```
- 需要修改的服务
```
master: kube-apiserver、kube-controller-manager、kubelet、kube-proxy、calico
node:   kubelet、kube-proxy、calico
```
- Master 操作步骤
```
# 1、修改 kube-apiserver 静态 pod 文件，追加启动特性，增加 svc IPv6 段地址。
$ vi /etc/kubernetes/manifests/kube-apiserver.yaml
spec:
  containers:
  - command:
  ...
    - --service-cluster-ip-range=10.96.0.0/12,fd00::/108
    - --feature-gates=IPv6DualStack=true

# 2、修改 kube-controller-manager 静态 pod 文件，追加启动特性，增加 svc IPv6 段地址，增加 pod IPv6 段地址。
$ vi /etc/kubernetes/manifests/kube-controller-manager.yaml
spec:
  containers:
  - command:
  ...
    - --cluster-cidr=172.16.0.0/16,fc00::/48
    - --service-cluster-ip-range=10.96.0.0/12,fd00::/108
    - --feature-gates=IPv6DualStack=true

# 3、修改 kubelet 文件，追加启动特性。
$ vi /etc/sysconfig/kubelet
KUBELET_EXTRA_ARGS="--feature-gates=IPv6DualStack=true"

# 4、修改 kube-proxy ComfigMap 文件，追加启动特性，增加 pod IPv6 段地址。
$ kubectl edit cm -n kube-system  kube-proxy
data:
  config.conf: |-
  ...
    featureGates:
      IPv6DualStack: true
    clusterCIDR: 172.16.0.0/16,fc00::/48

# 5、修改 calico.yaml 文件，追加 IPv6 相关信息，修改重新 apply。
$ vi calico.yaml
data:
  ...
  cni_network_config: |-
    {
      ...
        {
          ...
          "ipam": {
              "type": "calico-ipam",
              "assign_ipv4": "true",
              "assign_ipv6": "true"

spec:
  ...
  template:
    ...
    spec:
      ...
      containers:
        ...
        - name: calico-node
          ...
          env:
            - name: CALICO_IPV4POOL_CIDR
              value: "172.16.0.0/16"
            - name: IP6
              value: "autodetect"
            - name: CALICO_IPV6POOL_CIDR
              value: "fc00::/48"
            ...
            - name: FELIX_IPV6SUPPORT
              value: "true"               
```
- Node 操作步骤
```
# 1、修改 kubelet 文件，追加启动特性。
$ vi /etc/sysconfig/kubelet
KUBELET_EXTRA_ARGS="--feature-gates=IPv6DualStack=true"
```
- 重启相关服务
```
kube-apiserver、kube-controller-manager、kube-proxy、calico、kubelet
```
- 查看服务与访问
```
# 查看 pod 显示还是 v4 地址，因为静态 pod 里面 --advertise-address 是 v4 地址，如果改成 v6 创建 pod 出来的就是 v6 地址，但是创建 pod 两个 ip 都会有。 
# 创建 svc 需要 v6 地址就需要参数 ipFamilies 为 IPv6，这个时候出现的是 v6 地址，ep 也是 v6 地址。
# 如果设置了 NodePort，外部设备只能通过 v6 访问 v6 地址、v4 访问 v4 地址。
# 如果需要访问 svc 需要些路由，不然无法访问。

$ kubectl get pod -A -o wide
NAMESPACE     NAME                                       READY   STATUS    RESTARTS   AGE    IP               NODE     NOMINATED NODE   READINESS GATES
default       nginx-deployment1-585449566-48k9x          1/1     Running   0          159m   172.16.196.133   node01   <none>           <none>
default       nginx-deployment1-585449566-kwcnb          1/1     Running   0          159m   172.16.196.134   node01   <none>           <none>

$ kubectl describe pod nginx-deployment1-585449566-48k9x
Name:         nginx-deployment1-585449566-48k9x
...
IP:           172.16.196.133
IPs:
  IP:           172.16.196.133
  IP:           fc00::c4ae:fe9b:5aae:bdcd:3704

$ cat svc.yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-deployment1
spec:
  ipFamilies:
  - IPv6
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: ClusterIP

$ kubectl apply -f svc.yaml

$ kubectl get svc
NAME                TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
kubernetes          ClusterIP   10.96.0.1        <none>        443/TCP        3h5m
nginx-deployment    NodePort    10.104.153.146   <none>        80:30680/TCP   176m
nginx-deployment1   ClusterIP   fd00::e5e        <none>        80/TCP         4s

$ kubectl describe svc nginx-deployment1
Name:              nginx-deployment1
...
IP:                fd00::e5e
IPs:               fd00::e5e
Port:              <unset>  80/TCP
TargetPort:        80/TCP
Endpoints:         [fc00::c4ae:fe9b:5aae:bdcd:3704]:80,[fc00::c4ae:fe9b:5aae:bdcd:3705]:80
...

$ sudo route -A inet6 add ::/0 gw 2407:c080:802:bba::1
$ curl -6 http://[fd00::e5e]:80 -g
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
```