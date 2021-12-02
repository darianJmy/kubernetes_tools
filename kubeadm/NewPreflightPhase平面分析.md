# NewPreflightPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go 建议先看看cobra
##### NewPreflighPhase平面主要工作是检查环境

##### 1、Preflight 平面没有涉及子平面，运行内容全在 runPreflight 函数中
```
return workflow.Phase{
		Name:    "preflight",
		Short:   "Run pre-flight checks",
		Long:    "Run pre-flight checks for kubeadm init.",
		Example: preflightExample,
		Run:     runPreflight,
		InheritFlags: []string{
			options.CfgPath,
			options.IgnorePreflightErrors,
		},
	}
```

##### 2、runPreflight 函数分为两大块，一块检测系统方面是否准备就绪，一块检测镜像是否准备就绪
```
if err := preflight.RunInitNodeChecks(utilsexec.New(), data.Cfg(), data.IgnorePreflightErrors(), false, false); err != nil {
		return err
	}

if !data.DryRun() {
		fmt.Println("[preflight] Pulling images required for setting up a Kubernetes cluster")
		fmt.Println("[preflight] This might take a minute or two, depending on the speed of your internet connection")
		fmt.Println("[preflight] You can also perform this action in beforehand using 'kubeadm config images pull'")
		if err := preflight.RunPullImagesCheck(utilsexec.New(), data.Cfg(), data.IgnorePreflightErrors()); err != nil {
			return err
		}
	} else {
		fmt.Println("[preflight] Would pull the required images (like 'kubeadm config images pull')")
	}
```

##### 3、系统检测方面主要检测了以下部分
```
if !isSecondaryControlPlane {
		// First, check if we're root separately from the other preflight checks and fail fast
		//判断是否是特权用户，不然快速失败
		if err := RunRootCheckOnly(ignorePreflightErrors); err != nil {
			return err
		}
	}


manifestsDir := filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.ManifestsSubDirName)
    // 主要是收集信息到数组里面，最后执行 RunChecks 函数检测
    // 大概收集了一下信息
    // 1、cpu等于2、
    // 2、内存等于1700、
    // 3、待安装的 k8s 版本是否被当前版本的 kubeadm 支持（kubeadm 版本 >= 待安装 k8s 版本）
    // 4、LocalAPIEndpoint、KubeletPort、KubeSchedulerPort、KubeControllerManagerPort
    // 5、KubeAPIServer、KubeControllerManager、KubeScheduler、Etcd，通过pathjoin把manifestsDir和静态pod连接成string类型
    // 6、HTTP代理，连接本机网络、服务网络、Pod网络
    // 7、容器运行时信息
    // 7.1、如果容器运行时是docker
    // 8、如果机器不是linux不检查8.1-8.4
    // 8.1、收集crictl这个命令、
    // 8.2、收集/proc/sys/net/bridge/bridge-nf-call-iptables、/proc/sys/net/bridge/bridge-nf-call-iptables
	// 8.3、执行交换分区/proc/swaps、
    // 8.4、收集conntrack、ip、iptables、mount、nsenter、ebtables、ethtool、socat、tc、touch
    // 8.5、收集内核是否被支持Docker版本及后端存储 GraphDriver 是否被支持，对于 Linux 系统，还需检查 OS 版本和 cgroup 支持程度（支持哪些资源的隔离)
    // 8.6、收集主机名
    // 8.7、收集待安装的 kubelet 版本是否被当前版本的 kubeadm 支持（kubeadm 版本 >= 待安装 kubelet 版本）
    // 8.8、收集 kubelet 服务
    // 8.9、收集 kubelet 端口
    // 9、收集 Bridge-netfilter 和 IPv6 相关标志、
    // 10、如果使用外部 etcd、收集在创建集群之前检查外部 etcd 版本、
    // 11、收集需要安装本地 etcd 时才进行与 etcd 相关的检查、检查端口是否已被占用，EtcdListenClientPort、EtcdListenPort、
    // 12、收集使用外部 etcd 且不加入自动下载证书时检查 etcd 证书、检查外部etcd的CA、Cert、Key
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
	.............

// 运行 RunChecks 检测
RunChecks(checks, os.Stderr, ignorePreflightErrors)

// RunCheck 函数对数组进行循环，每个结构体都是有name、Check方法的，对每个结构体执行相应的Check方法进行安装前检测
func RunChecks(checks []Checker, ww io.Writer, ignorePreflightErrors sets.String) error {
	var errsBuffer bytes.Buffer

	for _, c := range checks {
		name := c.Name()
		warnings, errs := c.Check()

		if setHasItemOrAll(ignorePreflightErrors, name) {
			// Decrease severity of errors to warnings for this check
			warnings = append(warnings, errs...)
			errs = []error{}
		}

		for _, w := range warnings {
			io.WriteString(ww, fmt.Sprintf("\t[WARNING %s]: %v\n", name, w))
		}
		for _, i := range errs {
			errsBuffer.WriteString(fmt.Sprintf("\t[ERROR %s]: %v\n", name, i.Error()))
		}
	}
	if errsBuffer.Len() > 0 {
		return &Error{Msg: errsBuffer.String()}
	}
	return nil
}

// 以检测主机CPU为例
// Name returns the label for NumCPUCheck
func (NumCPUCheck) Name() string {
	return "NumCPU"
}

// Check number of CPUs required by kubeadm
func (ncc NumCPUCheck) Check() (warnings, errorList []error) {
	numCPU := runtime.NumCPU()
	if numCPU < ncc.NumCPU {
		errorList = append(errorList, errors.Errorf("the number of available CPUs %d is less than the required %d", numCPU, ncc.NumCPU))
	}
	return warnings, errorList
}
```

##### 3、镜像检测方面主要检测了以下部分
```
	// 主要是收集信息到数组里面，最后执行 RunChecks 函数检测
	// 这边有个 cfg.NodeRegistration.ImagePullPolicy 变量，主要是init执行这些平面前有一个初始化默认配置的步骤
	checks := []Checker{
			ImagePullCheck{runtime: containerRuntime, imageList: images.GetControlPlaneImages(&cfg.ClusterConfiguration), imagePullPolicy: cfg.NodeRegistration.ImagePullPolicy},
		}

// Name returns the label for ImagePullCheck
func (ImagePullCheck) Name() string {
	return "ImagePull"
}

// Check pulls images required by kubeadm. This is a mutating check
func (ipc ImagePullCheck) Check() (warnings, errorList []error) {
	policy := ipc.imagePullPolicy
	klog.V(1).Infof("using image pull policy: %s", policy)
	for _, image := range ipc.imageList {
		switch policy {
		case v1.PullNever:
			klog.V(1).Infof("skipping pull of image: %s", image)
			continue
		case v1.PullIfNotPresent:
			ret, err := ipc.runtime.ImageExists(image)
			if ret && err == nil {
				klog.V(1).Infof("image exists: %s", image)
				continue
			}
			if err != nil {
				errorList = append(errorList, errors.Wrapf(err, "failed to check if image %s exists", image))
			}
			fallthrough // Proceed with pulling the image if it does not exist
		case v1.PullAlways:
			klog.V(1).Infof("pulling: %s", image)
			if err := ipc.runtime.PullImage(image); err != nil {
				errorList = append(errorList, errors.Wrapf(err, "failed to pull image %s", image))
			}
		default:
			// If the policy is unknown return early with an error
			errorList = append(errorList, errors.Errorf("unsupported pull policy %q", policy))
			return warnings, errorList
		}
	}
	return warnings, errorList
}		
```
