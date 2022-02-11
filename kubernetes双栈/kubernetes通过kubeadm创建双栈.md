# Kubeadm 安装 Kubernetes 1.20.1 版本支持双栈功能的集群。
- 先决条件
```
为了使用 IPv4/IPv6 双栈的 Kubernetes 集群，需要满足以下先决条件：

Kubernetes 1.20 版本或更高版本，有关更早 Kubernetes 版本的使用双栈服务的信息， 请参考对应版本的 Kubernetes 文档。
提供商支持双协议栈网络（云提供商或其他提供商必须能够为 Kubernetes 节点提供可路由的 IPv4/IPv6 网络接口）。
支持双协议栈的网络插件（如 Kubenet 或 Calico）。

默认从 1.21 版本开始，IPv4/IPv6 双协议栈默认是被启用的，低于此版本的要开启双栈特性 IPv6DualStack。
```
- 需要的组件
```
master: kubeadm、kubelet、kubectl、docker、calico
node:   kubeadm、kubelet、docker、calico
```
- 需要注意的端口

控制平面节点 
| 协议 | 方向 | 端口范围 | 作用 |  使用者|
| --- | --- | --- | --- | --- |
| TCP | 入站 | 6443 | Kubernetes API 服务器 | 所有组件 |
| TCP | 入站 | 2379-2380 | etcd 服务器客户端 API | kube-apiserver, etcd |
| TCP | 入站 | 10250 | Kubelet API | kubelet 自身、控制平面组件 |
| TCP | 入站 | 10251 | kube-scheduler | kube-scheduler 自身 |
| TCP | 入站 | 10252 | kube-controller-manager | kube-controller-manager 自身 |

工作节点
| 协议 | 方向 | 端口范围 | 作用 |  使用者|
| --- | --- | --- | --- | --- |
| TCP | 入站 | 10250 | Kubelet API | kubelet 自身、控制平面组件 |
| TCP | 入站 | 30000-32767 | NodePort 服务† | 所有组件 |
 

- 单节点安装步骤

确保 `br_netfilter` 模块被加载。这一操作可以通过运行 `lsmod | grep br_netfilter` 来完成。若要显式加载该模块，可执行 `sudo modprobe br_netfilter`。

为了让你的 `Linux` 节点上的 `iptables` 能够正确地查看桥接流量，你需要确保在你的 `sysctl` 配置中将 `net.bridge.bridge-nf-call-iptables` 设置为 `1`。
```
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
br_netfilter
EOF

cat <<EOF | sudo tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
EOF
sudo sysctl --system
```
  - 关闭防火墙和`selinux`、修改主机名
```
sudo systemctl stop firewalld
sudo systemctl disable firewalld
sudo setenforce 0
sudo sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config
hostnamectl set-hostname master
```  
  - 配置成国内源
```  
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://mirrors.aliyun.com/kubernetes/yum/repos/kubernetes-el7-x86_64/
enabled=1
gpgcheck=0
repo_gpgcheck=0
gpgkey=https://mirrors.aliyun.com/kubernetes/yum/doc/yum-key.gpghttps://mirrors.aliyun.com/kubernetes/yum/doc/rpm-package-key.gpg
EOF

cat <<EOF > /etc/yum.repos.d/docker-ce.repo
[docker-ce-stable]
name=Docker CE Stable - $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/$basearch/stable
enabled=1
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-stable-debuginfo]
name=Docker CE Stable - Debuginfo $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/debug-$basearch/stable
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-stable-source]
name=Docker CE Stable - Sources
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/source/stable
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-edge]
name=Docker CE Edge - $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/$basearch/edge
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-edge-debuginfo]
name=Docker CE Edge - Debuginfo $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/debug-$basearch/edge
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-edge-source]
name=Docker CE Edge - Sources
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/source/edge
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-test]
name=Docker CE Test - $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/$basearch/test
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-test-debuginfo]
name=Docker CE Test - Debuginfo $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/debug-$basearch/test
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-test-source]
name=Docker CE Test - Sources
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/source/test
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-nightly]
name=Docker CE Nightly - $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/$basearch/nightly
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-nightly-debuginfo]
name=Docker CE Nightly - Debuginfo $basearch
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/debug-$basearch/nightly
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg

[docker-ce-nightly-source]
name=Docker CE Nightly - Sources
baseurl=https://mirrors.aliyun.com/docker-ce/linux/centos/7/source/nightly
enabled=0
gpgcheck=1
gpgkey=https://mirrors.aliyun.com/docker-ce/linux/centos/gpg


EOF
```
  - 控制节点安装服务组件
```
yum install -y kubelet-1.20.1 kubeadm-1.20.1 kubectl-1.20.1 docker-ce
```
  - 配置`daemon.json`与启动`docker`
```
cat <<EOF > /etc/docker/daemon.json
{
  "registry-mirrors": ["https://hdi5v8p1.mirror.aliyuncs.com"],
  "insecure-registries": ["0.0.0.0/0"],
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m"
  }
}
EOF
sudo systemctl restart docker
sudo systemctl enable docker
```  
  - 获取通过国内源获取镜像
```
kubeadm config images list --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.20.1
kubeadm config images pull --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.20.1
```
  - `kubeadm`初始化
```  
kubeadm init  --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.20.1  --pod-network-cidr="172.16.0.0/16,fc00::/48"  --feature-gates IPv6DualStack=true --service-cidr="10.96.0.0/12,fd00::/108"
```
  - 配置`config`文件
```
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```  
  - 安装网络插件
```
curl https://docs.projectcalico.org/manifests/calico.yaml -O
vi calico.yaml
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
- 现在`kubectl get pod -A`发现所有组件全部安装成功
```
[root@master ~]# kubectl get pod -A
NAMESPACE     NAME                                       READY   STATUS    RESTARTS   AGE
kube-system   calico-kube-controllers-558995777d-bszkw   1/1     Running   0          3h9m
kube-system   calico-node-4mfjl                          1/1     Running   0          3h9m
kube-system   calico-node-ksgvt                          1/1     Running   0          3h9m
kube-system   coredns-54d67798b7-nv2c7                   1/1     Running   0          3h27m
kube-system   coredns-54d67798b7-wd7nz                   1/1     Running   0          78m
kube-system   etcd-master                                1/1     Running   0          3h27m
kube-system   kube-apiserver-master                      1/1     Running   0          3h16m
kube-system   kube-controller-manager-master             1/1     Running   0          3h16m
kube-system   kube-proxy-rd2tl                           1/1     Running   0          3h12m
kube-system   kube-proxy-zbbq4                           1/1     Running   0          3h12m
kube-system   kube-scheduler-master                      1/1     Running   1          3h27m
```