# Kubeadm源码分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go 建议先看看cobra
##### kubeadm 在执行 init 的主要分为13个平面，分别为检查环境、创建CA、创建Config文件、启动Kubelet、创建静态pod文件、创建Etcd静态文件、等待Kubelet拉起静态pod文件、上传Config文件到ConfigMap中、上传Certs到Secret中、打标签与污点、创建Token到Secret中、替换kubelet里的pem文件目录、创建CoreDNS和Proxy。
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
initRunner.AppendPhase(phases.NewAddonPhase())
```
##### 1、在NewPreflightPhase函数里找到workflow.Phase结构体里面的Run对应的runPreflight函数，主要任务在RunInitNodeChecks与RunPullImagesCheck函数里
```
if err := preflight.RunInitNodeChecks(utilsexec.New(), data.Cfg(), data.IgnorePreflightErrors(), false, false); err != nil {
		return err
	}

if err := preflight.RunPullImagesCheck(utilsexec.New(), data.Cfg(), data.IgnorePreflightErrors()); err != nil {
			return err
		}
	} else {
		fmt.Println("[preflight] Would pull the required images (like 'kubeadm config images pull')")
	}    
```
###### RunInitNodeChecks函数主要检查了下面的项目：
```
    1、检查cpu是否小于2、
    2、检查内存是否小于1700、
    3、检查待安装的 k8s 版本是否被当前版本的 kubeadm 支持（kubeadm 版本 >= 待安装 k8s 版本）
    4、检查防火墙服务是否存在，如果防火墙未关闭，提示开放端口LocalAPIEndpoint、KubeletPort
    5、检查端口是否已被占用，LocalAPIEndpoint、KubeSchedulerPort、KubeControllerManagerPort
    6、检查/etc/kubernetes/manifests/下文件是否已经存在，KubeAPIServer、KubeControllerManager、KubeScheduler、Etcd
    7、检查是否存在代理，连接本机网络、服务网络、Pod网络，都会检查，目前不允许代理
    8、进行系统检查
    8.1、检查容器运行时是否是running
    8.2、如果容器运行时docker检查服务是否可用
    //如果机器不是linux不检查8.3-8.6
    8.3、检查有没有crictl这个命令、
    8.4、检查/proc/sys/net/bridge/bridge-nf-call-iptables、/proc/sys/net/bridge/bridge-nf-call-iptables配置
    8.5、检查交换分区有没有关闭/proc/swaps、
    8.6、检查有没有conntrack、ip、iptables、mount、nsenter、ebtables、ethtool、socat、tc、touch命令
    8.7、检查内核是否被支持Docker版本及后端存储 GraphDriver 是否被支持，对于 Linux 系统，还需检查 OS 版本和 cgroup 支持程度（支持哪些资源的隔离)
    8.8、检查主机名访问可达性
    8.9、检查待安装的 kubelet 版本是否被当前版本的 kubeadm 支持（kubeadm 版本 >= 待安装 kubelet 版本）
    8.10、检查 kubelet 服务是否开机自启动
    8.11、检查 kubelet 端口是否被占用
    9、检查是否设置了 Bridge-netfilter 和 IPv6 相关标志、
    10、如果使用外部 etcd、在创建集群之前检查外部 etcd 版本、
    11、仅在需要安装本地 etcd 时才进行与 etcd 相关的检查、检查端口是否已被占用，EtcdListenClientPort、EtcdListenPort、
    12、仅在使用外部 etcd 且不加入自动下载证书时检查 etcd 证书、检查外部etcd的CA、Cert、Key
```
```
manifestsDir := filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.ManifestsSubDirName)
	checks := []Checker{
		NumCPUCheck{NumCPU: kubeadmconstants.ControlPlaneNumCPU},
		// Linux only
		// TODO: support other OS, if control-plane is supported on it.
		MemCheck{Mem: kubeadmconstants.ControlPlaneMem},
		KubernetesVersionCheck{KubernetesVersion: cfg.KubernetesVersion, KubeadmVersion: kubeadmversion.Get().GitVersion},
		FirewalldCheck{ports: []int{int(cfg.LocalAPIEndpoint.BindPort), kubeadmconstants.KubeletPort}},
		PortOpenCheck{port: int(cfg.LocalAPIEndpoint.BindPort)},
		PortOpenCheck{port: kubeadmconstants.KubeSchedulerPort},
		PortOpenCheck{port: kubeadmconstants.KubeControllerManagerPort},
		FileAvailableCheck{Path: kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeAPIServer, manifestsDir)},
		FileAvailableCheck{Path: kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeControllerManager, manifestsDir)},
		FileAvailableCheck{Path: kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.KubeScheduler, manifestsDir)},
		FileAvailableCheck{Path: kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.Etcd, manifestsDir)},
		HTTPProxyCheck{Proto: "https", Host: cfg.LocalAPIEndpoint.AdvertiseAddress},
	}
	cidrs := strings.Split(cfg.Networking.ServiceSubnet, ",")
	for _, cidr := range cidrs {
		checks = append(checks, HTTPProxyCIDRCheck{Proto: "https", CIDR: cidr})
	}
	cidrs = strings.Split(cfg.Networking.PodSubnet, ",")
	for _, cidr := range cidrs {
		checks = append(checks, HTTPProxyCIDRCheck{Proto: "https", CIDR: cidr})
	}

	if !isSecondaryControlPlane {
		checks = addCommonChecks(execer, cfg.KubernetesVersion, &cfg.NodeRegistration, checks)

		// Check if Bridge-netfilter and IPv6 relevant flags are set
		if ip := netutils.ParseIPSloppy(cfg.LocalAPIEndpoint.AdvertiseAddress); ip != nil {
			if netutils.IsIPv6(ip) {
				checks = append(checks,
					FileContentCheck{Path: bridgenf6, Content: []byte{'1'}},
					FileContentCheck{Path: ipv6DefaultForwarding, Content: []byte{'1'}},
				)
			}
		}

		// if using an external etcd
		if cfg.Etcd.External != nil {
			// Check external etcd version before creating the cluster
			checks = append(checks, ExternalEtcdVersionCheck{Etcd: cfg.Etcd})
		}
	}

	if cfg.Etcd.Local != nil {
		// Only do etcd related checks when required to install a local etcd
		checks = append(checks,
			PortOpenCheck{port: kubeadmconstants.EtcdListenClientPort},
			PortOpenCheck{port: kubeadmconstants.EtcdListenPeerPort},
			DirAvailableCheck{Path: cfg.Etcd.Local.DataDir},
		)
	}

	if cfg.Etcd.External != nil && !(isSecondaryControlPlane && downloadCerts) {
		// Only check etcd certificates when using an external etcd and not joining with automatic download of certs
		if cfg.Etcd.External.CAFile != "" {
			checks = append(checks, FileExistingCheck{Path: cfg.Etcd.External.CAFile, Label: "ExternalEtcdClientCertificates"})
		}
		if cfg.Etcd.External.CertFile != "" {
			checks = append(checks, FileExistingCheck{Path: cfg.Etcd.External.CertFile, Label: "ExternalEtcdClientCertificates"})
		}
		if cfg.Etcd.External.KeyFile != "" {
			checks = append(checks, FileExistingCheck{Path: cfg.Etcd.External.KeyFile, Label: "ExternalEtcdClientCertificates"})
		}
	}

```
```
containerRuntime, err := utilruntime.NewContainerRuntime(execer, nodeReg.CRISocket)
	isDocker := false
	if err != nil {
		fmt.Printf("[preflight] WARNING: Couldn't create the interface used for talking to the container runtime: %v\n", err)
	} else {
		checks = append(checks, ContainerRuntimeCheck{runtime: containerRuntime})
		if containerRuntime.IsDocker() {
			isDocker = true
			checks = append(checks, ServiceCheck{Service: "docker", CheckIfActive: true})
		}
	}

	// non-windows checks
	if runtime.GOOS == "linux" {
		if !isDocker {
			checks = append(checks, InPathCheck{executable: "crictl", mandatory: true, exec: execer})
		}
		checks = append(checks,
			FileContentCheck{Path: bridgenf, Content: []byte{'1'}},
			FileContentCheck{Path: ipv4Forward, Content: []byte{'1'}},
			SwapCheck{},
			InPathCheck{executable: "conntrack", mandatory: true, exec: execer},
			InPathCheck{executable: "ip", mandatory: true, exec: execer},
			InPathCheck{executable: "iptables", mandatory: true, exec: execer},
			InPathCheck{executable: "mount", mandatory: true, exec: execer},
			InPathCheck{executable: "nsenter", mandatory: true, exec: execer},
			InPathCheck{executable: "ebtables", mandatory: false, exec: execer},
			InPathCheck{executable: "ethtool", mandatory: false, exec: execer},
			InPathCheck{executable: "socat", mandatory: false, exec: execer},
			InPathCheck{executable: "tc", mandatory: false, exec: execer},
			InPathCheck{executable: "touch", mandatory: false, exec: execer})
	}
	checks = append(checks,
		SystemVerificationCheck{IsDocker: isDocker},
		HostnameCheck{nodeName: nodeReg.Name},
		KubeletVersionCheck{KubernetesVersion: k8sVersion, exec: execer},
		ServiceCheck{Service: "kubelet", CheckIfActive: false},
		PortOpenCheck{port: kubeadmconstants.KubeletPort})
```
