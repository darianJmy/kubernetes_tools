# Kubeadm源码分析
##### 分析版本为1.22
## kubeadm 在执行 init 的主要分为13个平面，分别为检查环境、创建CA、创建Config文件、启动Kubelet、创建静态pod文件、创建Etcd静态文件、等待Kubelet拉起静态pod文件、上传Config文件到ConfigMap中、上传Certs到Secret中、打标签与污点、创建Token到Secret中、替换kubelet里的pem文件目录、创建CoreDNS和Proxy。
```
	// initialize the workflow runner with the list of phases
	initRunner.AppendPhase(phases.NewPreflightPhase())
	initRunner.AppendPhase(phases.NewCertsPhase())
	initRunner.AppendPhase(phases.NewKubeConfigPhase())
	initRunner.AppendPhase(phases.NewKubeletStartPhase())
	initRunner.AppendPhase(phases.NewControlPlanePhase())
	initRunner.AppendPhase(phases.NewEtcdPhase())
	initRunner.AppendPhase(phases.NewWaitControlPlanePhase())
	initRunner.AppendPhase(phases.NewUploadConfigPhase())
	initRunner.AppendPhase(phases.NewUploadCertsPhase())
	initRunner.AppendPhase(phases.NewMarkControlPlanePhase())
	initRunner.AppendPhase(phases.NewBootstrapTokenPhase())
	initRunner.AppendPhase(phases.NewKubeletFinalizePhase())
	initRunner.AppendPhase(phases.NewAddonPhase())，主要包含：环境检测、配置加载集群初始化、安装后配置等步骤。
```
