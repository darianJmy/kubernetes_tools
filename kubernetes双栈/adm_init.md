kubeadm init  --image-repository registry.cn-hangzhou.aliyuncs.com/google_containers  --kubernetes-version 1.20.1  --pod-network-cidr="172.16.0.0/16,fc00::/48"  --feature-gates IPv6DualStack=true --service-cidr="10.96.0.0/12,fd00::/108"


ip -6 add add 2407:c080:802:bba:887f:a699:709d:ad00/64 dev eth0
sudo route -A inet6 add ::/0 gw 2407:c080:802:bba::1
curl https://docs.projectcalico.org/manifests/calico.yaml -O
