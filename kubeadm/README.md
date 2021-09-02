# 通过Kubeadm安装Kubernetes集群

- 需要的组件
```
master: kubeadm、kubelet、kubectl、docker
node:   kubeadm、kubelet、docker
```
- 需要注意的端口

控制平面节点 
```
| 协议   | 方向  | 端口范围  | 作用  | 使用者 |
| :---- | :---- | :----   | :---- | :---- |
| TCP	  |入站	  |6443     |	Kubernetes API |服务器	所有组件 |
| TCP	  |入站	  |2379-2380 |	etcd |服务器客户端 API	kube-apiserver, etcd |
| TCP	  |入站	  |10250	    |Kubelet API	| kubelet 自身、控制平面组件 |
| TCP	  |入站	  |10251	    |kube-scheduler	| kube-scheduler 自身 |
| TCP   |入站	  |10252	    |kube-controller-manager	| kube-controller-manager 自身 |
```
工作节点
```
协议	方向	端口范围	     作用	         使用者
TCP	入站	10250	      Kubelet API	   kubelet 自身、控制平面组件
TCP	入站	30000-32767	NodePort 服务†	 所有组件
```  

- 单节点安装步骤

确保 br_netfilter 模块被加载。这一操作可以通过运行 lsmod | grep br_netfilter 来完成。若要显式加载该模块，可执行 sudo modprobe br_netfilter。

为了让你的 Linux 节点上的 iptables 能够正确地查看桥接流量，你需要确保在你的 sysctl 配置中将 net.bridge.bridge-nf-call-iptables 设置为 1。
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

cat <<EOF > /etc/yum.repos.d/kubernetes.repo
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
yum install -y kubelet-1.18.6 kubeadm-1.18.6 kubectl-1.18.6 docker
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
kubeadm config images list --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.18.6
kubeadm config images pull --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.18.6
```
  - `kubeadm`初始化
```  
kubeadm init  --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.18.6
```
  - 配置`config`文件
```
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```  
  - 安装网络插件
```
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
```
- 这个时候`kubectl get pod -A`会发现flannel没有子网，需要加个patch
```
kubectl patch node master -p '{"spec":{"podCIDR":"10.244.0.0/16"}}'
```  
- 现在`kubectl get pod -A`发现所有组件全部安装成功
```
[root@master ~]# kubectl get pod -A
NAMESPACE     NAME                             READY   STATUS    RESTARTS   AGE
default       nginx-76d984d5c7-xm7kd           1/1     Running   0          68m
kube-system   coredns-546565776c-bkbnp         1/1     Running   0          8h
kube-system   coredns-546565776c-mmvnv         1/1     Running   0          8h
kube-system   etcd-master                      1/1     Running   0          8h
kube-system   kube-apiserver-master            1/1     Running   1          8h
kube-system   kube-controller-manager-master   1/1     Running   0          7h56m
kube-system   kube-flannel-ds-2q9gw            1/1     Running   5          7h59m
kube-system   kube-proxy-xdfnb                 1/1     Running   0          8h
kube-system   kube-scheduler-master            1/1     Running   1          8h
```

- TODO: `patch`的子网应该在`kubeadm init`时带入进去，需要研究kubeadm init参数与自定义参数，达到效果跟二进制安装无差别
