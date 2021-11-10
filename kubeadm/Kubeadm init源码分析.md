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
###### RunPullImagesCheck函数主要检查了image是否存在
```
checks := []Checker{
		ImagePullCheck{runtime: containerRuntime, imageList: images.GetControlPlaneImages(&cfg.ClusterConfiguration), imagePullPolicy: cfg.NodeRegistration.ImagePullPolicy},
	}
```
##### 2、在NewCertsPhase函数里又定义了多个平面，各个平面对应着各个组件的查看证书与创建的过程
```
return workflow.Phase{
		Name:   "certs",
		Short:  "Certificate generation",
		Phases: newCertSubPhases(),
		Run:    runCerts,
		Long:   cmdutil.MacroCommandLongDescription,
	}

return Certificates{
        //各个证书内容都以定义成函数
		KubeadmCertRootCA(),
		KubeadmCertAPIServer(),
		KubeadmCertKubeletClient(),
		// Front Proxy certs
		KubeadmCertFrontProxyCA(),
		KubeadmCertFrontProxyClient(),
		// etcd certs
		KubeadmCertEtcdCA(),
		KubeadmCertEtcdServer(),
		KubeadmCertEtcdPeer(),
		KubeadmCertEtcdHealthcheck(),
		KubeadmCertEtcdAPIClient(),
	}     
```
###### 先是检查各个组件ca是否存在如果不存在就创建
```
for _, cert := range certsphase.GetDefaultCertList() {
		var phase workflow.Phase
        检查组件里列表里是否有CAName进行分类处理
		if cert.CAName == "" {
			phase = newCertSubPhase(cert, runCAPhase(cert))
			lastCACert = cert
		} else {
			phase = newCertSubPhase(cert, runCertPhase(cert, lastCACert))
		}
		subPhases = append(subPhases, phase)
	}

//newCertSubPhase函数
//先检查是否存
if certData, intermediates, err := pkiutil.TryLoadCertChainFromDisk(data.CertificateDir(), cert.BaseName); err == nil {
			certsphase.CheckCertificatePeriodValidity(cert.BaseName, certData)

			caCertData, err := pkiutil.TryLoadCertFromDisk(data.CertificateDir(), caCert.BaseName)
			if err != nil {
				return errors.Wrapf(err, "couldn't load CA certificate %s", caCert.Name)
			}

			certsphase.CheckCertificatePeriodValidity(caCert.BaseName, caCertData)

			if err := pkiutil.VerifyCertChain(certData, intermediates, caCertData); err != nil {
				return errors.Wrapf(err, "[certs] certificate %s not signed by CA certificate %s", cert.BaseName, caCert.BaseName)
			}

			fmt.Printf("[certs] Using existing %s certificate and key on disk\n", cert.BaseName)
			return nil
		}

		// if dryrunning, write certificates to a temporary folder (and defer restore to the path originally specified by the user)
		cfg := data.Cfg()
		cfg.CertificatesDir = data.CertificateWriteDir()
		defer func() { cfg.CertificatesDir = data.CertificateDir() }()

		// create the new certificate (or use existing)
        //如果不存在则创建
		return certsphase.CreateCertAndKeyFilesWithCA(cert, caCert, cfg)
```
##### 3、在NewKubeConfigPhase函数里又定义了多个平面，各个平面对应着config文件的创建
```
return workflow.Phase{
		Name:  "kubeconfig",
		Short: "Generate all kubeconfig files necessary to establish the control plane and the admin kubeconfig file",
		Long:  cmdutil.MacroCommandLongDescription,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Generate all kubeconfig files",
				InheritFlags:   getKubeConfigPhaseFlags("all"),
				RunAllSiblings: true,
			},
			NewKubeConfigFilePhase(kubeadmconstants.AdminKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.KubeletKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.ControllerManagerKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.SchedulerKubeConfigFileName),
		},
		Run: runKubeConfig,
	}
```
###### 检查是否是外部CA，是外部CA尝试写入
```
if data.ExternalCA() {
			fmt.Printf("[kubeconfig] External CA mode: Using user provided %s\n", kubeConfigFileName)
			// If using an external CA while dryrun, copy kubeconfig files to dryrun dir for later use
			if data.DryRun() {
				externalCAFile := filepath.Join(kubeadmconstants.KubernetesDir, kubeConfigFileName)
				fileInfo, _ := os.Stat(externalCAFile)
				contents, err := os.ReadFile(externalCAFile)
				if err != nil {
					return err
				}
				err = os.WriteFile(filepath.Join(data.KubeConfigDir(), kubeConfigFileName), contents, fileInfo.Mode())
				if err != nil {
					return err
				}
			}
			return nil
		}

	
```
###### 创建Config文件并写入配置
```
// creates the KubeConfig file (or use existing)
return kubeconfigphase.CreateKubeConfigFile(kubeConfigFileName, data.KubeConfigDir(), data.Cfg())
```
##### 4、在NewKubeletStartPhase函数里找到workflow.Phase结构体里面的Run对应的runKubeletStart函数主要任务是关闭
```
//关闭Kubelet
if !data.DryRun() {
		klog.V(1).Infoln("Stopping the kubelet")
		kubeletphase.TryStopKubelet()
	}

//写入配置文件文件

if err := kubeletphase.WriteKubeletDynamicEnvFile(&data.Cfg().ClusterConfiguration, &data.Cfg().NodeRegistration, false, data.KubeletDir()); err != nil {
		return errors.Wrap(err, "error writing a dynamic environment file for the kubelet")
	}

if err := kubeletphase.WriteConfigToDisk(&data.Cfg().ClusterConfiguration, data.KubeletDir()); err != nil {
		return errors.Wrap(err, "error writing kubelet configuration to disk")
	}   

//启动kubelet
if !data.DryRun() {
		fmt.Println("[kubelet-start] Starting the kubelet")
		kubeletphase.TryStartKubelet()
	}     
```
###### 写入的配置文件为/var/lib/kubelet/kubeadm-flags.env、/var/lib/kubelet/config.yaml
```
kubeletEnvFilePath := filepath.Join(kubeletDir, constants.KubeletEnvFileName)
	fmt.Printf("[kubelet-start] Writing kubelet environment file with flags to file %q\n", kubeletEnvFilePath)

	// creates target folder if not already exists
	if err := os.MkdirAll(kubeletDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", kubeletDir)
	}
	if err := ioutil.WriteFile(kubeletEnvFilePath, b, 0644); err != nil {
		return errors.Wrapf(err, "failed to write kubelet configuration to the file %q", kubeletEnvFilePath)
	}


 configFile := filepath.Join(kubeletDir, kubeadmconstants.KubeletConfigurationFileName)
	fmt.Printf("[kubelet-start] Writing kubelet configuration to file %q\n", configFile)

	// creates target folder if not already exists
	if err := os.MkdirAll(kubeletDir, 0700); err != nil {
		return errors.Wrapf(err, "failed to create directory %q", kubeletDir)
	}

	if err := ioutil.WriteFile(configFile, b, 0644); err != nil {
		return errors.Wrapf(err, "failed to write kubelet configuration to the file %q", configFile)
	}   
```
##### 5、在NewControlPlanePhase函数里又定义了多个平面，各个平面对应着创建静态pod文件
```
phase := workflow.Phase{
		Name:  "control-plane",
		Short: "Generate all static Pod manifest files necessary to establish the control plane",
		Long:  cmdutil.MacroCommandLongDescription,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Generate all static Pod manifest files",
				InheritFlags:   getControlPlanePhaseFlags("all"),
				Example:        controlPlaneExample,
				RunAllSiblings: true,
			},
			newControlPlaneSubphase(kubeadmconstants.KubeAPIServer),
			newControlPlaneSubphase(kubeadmconstants.KubeControllerManager),
			newControlPlaneSubphase(kubeadmconstants.KubeScheduler),
		},
		Run: runControlPlanePhase,
	}
```
###### 创建静态Pod文件
```
data, ok := c.(InitData)
		if !ok {
			return errors.New("control-plane phase invoked with an invalid data struct")
		}
		cfg := data.Cfg()

		fmt.Printf("[control-plane] Creating static Pod manifest for %q\n", component)
		return controlplane.CreateStaticPodFiles(data.ManifestDir(), data.PatchesDir(), &cfg.ClusterConfiguration, &cfg.LocalAPIEndpoint, data.DryRun(), component)
	}

 //Specs
 staticPodSpecs := map[string]v1.Pod{
		kubeadmconstants.KubeAPIServer: staticpodutil.ComponentPod(v1.Container{
			Name:            kubeadmconstants.KubeAPIServer,
			Image:           images.GetKubernetesImage(kubeadmconstants.KubeAPIServer, cfg),
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         getAPIServerCommand(cfg, endpoint),
			VolumeMounts:    staticpodutil.VolumeMountMapToSlice(mounts.GetVolumeMounts(kubeadmconstants.KubeAPIServer)),
			LivenessProbe:   staticpodutil.LivenessProbe(staticpodutil.GetAPIServerProbeAddress(endpoint), "/livez", int(endpoint.BindPort), v1.URISchemeHTTPS),
			ReadinessProbe:  staticpodutil.ReadinessProbe(staticpodutil.GetAPIServerProbeAddress(endpoint), "/readyz", int(endpoint.BindPort), v1.URISchemeHTTPS),
			StartupProbe:    staticpodutil.StartupProbe(staticpodutil.GetAPIServerProbeAddress(endpoint), "/livez", int(endpoint.BindPort), v1.URISchemeHTTPS, cfg.APIServer.TimeoutForControlPlane),
			Resources:       staticpodutil.ComponentResources("250m"),
			Env:             kubeadmutil.GetProxyEnvVars(),
		}, mounts.GetVolumes(kubeadmconstants.KubeAPIServer),
			map[string]string{kubeadmconstants.KubeAPIServerAdvertiseAddressEndpointAnnotationKey: endpoint.String()}),


//通过WriteFile创建，在创建之前还要收集一些信息      
if err := ioutil.WriteFile(filename, serialized, 0600); err != nil {
		return errors.Wrapf(err, "failed to write static pod manifest file for %q (%q)", componentName, filename)
	}
```
##### 6、在NewEtcdPhase函数里又定义了一个平面为newEtcdLocalSubPhase
```
func newEtcdLocalSubPhase() workflow.Phase {
	phase := workflow.Phase{
		Name:         "local",
		Short:        "Generate the static Pod manifest file for a local, single-node local etcd instance",
		Example:      etcdLocalExample,
		Run:          runEtcdPhaseLocal(),
		InheritFlags: getEtcdPhaseFlags(),
	}
	return phase
}
```
###### 如果没有定义外部etcd则创建etcd数据目录和创建静态Pod文件，如果有外部etcd测跳过
```
if cfg.Etcd.External == nil {
			// creates target folder if doesn't exist already
			if !data.DryRun() {
				// Create the etcd data directory
				if err := etcdutil.CreateDataDirectory(cfg.Etcd.Local.DataDir); err != nil {
					return err
				}
			} else {
				fmt.Printf("[dryrun] Would ensure that %q directory is present\n", cfg.Etcd.Local.DataDir)
			}
			fmt.Printf("[etcd] Creating static Pod manifest for local etcd in %q\n", data.ManifestDir())
			if err := etcdphase.CreateLocalEtcdStaticPodManifestFile(data.ManifestDir(), data.PatchesDir(), cfg.NodeRegistration.Name, &cfg.ClusterConfiguration, &cfg.LocalAPIEndpoint, data.DryRun()); err != nil {
				return errors.Wrap(err, "error creating local etcd static pod manifest file")
			}
		} else {
			klog.V(1).Infoln("[etcd] External etcd mode. Skipping the creation of a manifest for local etcd")
		}

// writes etcd StaticPod to disk
	if err := staticpodutil.WriteStaticPodToDisk(kubeadmconstants.Etcd, manifestDir, spec); err != nil {
		return err
	}        
```
##### 7、在NewKubeletStartPhase函数里找到workflow.Phase结构体里面的Run对应的runWaitControlPlanePhase函数等待
```
phase := workflow.Phase{
		Name:   "wait-control-plane",
		Run:    runWaitControlPlanePhase,
		Hidden: true,
	}

// newControlPlaneWaiter returns a new waiter that is used to wait on the control plane to boot up.
func newControlPlaneWaiter(dryRun bool, timeout time.Duration, client clientset.Interface, out io.Writer) (apiclient.Waiter, error) {
	if dryRun {
		return dryrunutil.NewWaiter(), nil
	}

	return apiclient.NewKubeWaiter(client, timeout, out), nil
}
```
##### 8、在NewUploadConfigPhase函数里又定义了多个平面，为runUploadKubeadmConfig和runUploadKubeletConfig
```
//runUploadKubeadmConfig创建kubeadmcofigmap与rbac
klog.V(1).Infoln("[upload-config] Uploading the kubeadm ClusterConfiguration to a ConfigMap")
	if err := uploadconfig.UploadConfiguration(cfg, client); err != nil {
		return errors.Wrap(err, "error uploading the kubeadm ClusterConfiguration")
	}

err = apiclient.CreateOrMutateConfigMap(client, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmconstants.KubeadmConfigConfigMap,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			kubeadmconstants.ClusterConfigurationConfigMapKey: string(clusterConfigurationYaml),
		},
	}

// Ensure that the NodesKubeadmConfigClusterRoleName exists
err = apiclient.CreateOrUpdateRole(client, &rbac.Role{
	ObjectMeta: metav1.ObjectMeta{
		Name:      NodesKubeadmConfigClusterRoleName,
		Namespace: metav1.NamespaceSystem,
	},
	Rules: []rbac.PolicyRule{
		{
			Verbs:         []string{"get"},
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{kubeadmconstants.KubeadmConfigConfigMap},
		},
	},
})

//runUploadKubeletConfig创建kubeletcofigmap、rbac与node
klog.V(1).Infoln("[upload-config] Uploading the kubelet component config to a ConfigMap")
	if err = kubeletphase.CreateConfigMap(&cfg.ClusterConfiguration, client); err != nil {
		return errors.Wrap(err, "error creating kubelet configuration ConfigMap")
	}

klog.V(1).Infoln("[upload-config] Preserving the CRISocket information for the control-plane node")
	if err := patchnodephase.AnnotateCRISocket(client, cfg.NodeRegistration.Name, cfg.NodeRegistration.CRISocket); err != nil {
		return errors.Wrap(err, "Error writing Crisocket information for the control-plane node")
	}    
```
##### 9、在NewUploadCertsPhase函数里找到workflow.Phase结构体里面的Run对应的runUploadCerts
```
return workflow.Phase{
		Name:  "upload-certs",
		Short: fmt.Sprintf("Upload certificates to %s", kubeadmconstants.KubeadmCertsSecret),
		Long:  cmdutil.MacroCommandLongDescription,
		Run:   runUploadCerts,
		InheritFlags: []string{
			options.CfgPath,
			options.KubeconfigPath,
			options.UploadCerts,
			options.CertificateKey,
			options.SkipCertificateKeyPrint,
		},
	}
 
//这边为非data.UploadCerts，init一般不带UploadCerts参数所以默认不执行
if !data.UploadCerts() {
	fmt.Printf("[upload-certs] Skipping phase. Please see --%s\n", options.UploadCerts)
	return nil	
}    

//这边主要是通过client调用的CreateOrUpdateSecret函数，分别创建了secret与rbac。
err = apiclient.CreateOrUpdateSecret(client, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            kubeadmconstants.KubeadmCertsSecret,
			Namespace:       metav1.NamespaceSystem,
			OwnerReferences: ref,
		},
		Data: secretData,
	})
	if err != nil {
		return err
	}

	return createRBAC(client)

func CreateOrUpdateSecret(client clientset.Interface, secret *v1.Secret) error {
	if _, err := client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "unable to create secret")
		}

		if _, err := client.CoreV1().Secrets(secret.ObjectMeta.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "unable to update secret")
		}
	}
	return nil
}
```
##### 10、在NewMarkControlPlanePhase函数里找到workflow.Phase结构体里面的Run对应的runMarkControlPlane
```
return workflow.Phase{
		Name:    "mark-control-plane",
		Short:   "Mark a node as a control-plane",
		Example: markControlPlaneExample,
		InheritFlags: []string{
			options.NodeName,
			options.CfgPath,
		},
		Run: runMarkControlPlane,
	}
//标记控制平面
return markcontrolplanephase.MarkControlPlane(client, nodeRegistration.Name, nodeRegistration.Taints)

//apiclient对Node进行操作
return apiclient.PatchNode(client, controlPlaneName, func(n *v1.Node) {
		markControlPlaneNode(n, taints)
	})

func markControlPlaneNode(n *v1.Node, taints []v1.Taint) {
	for _, label := range labelsToAdd {
		n.ObjectMeta.Labels[label] = ""
	}

	for _, nt := range n.Spec.Taints {
		if !taintExists(nt, taints) {
			taints = append(taints, nt)
		}
	}

	n.Spec.Taints = taints
}
```
##### 11、在NewBootstrapTokenPhase函数里找到workflow.Phase结构体里面的Run对应的runBootstrapToken
```
return workflow.Phase{
		Name:    "bootstrap-token",
		Aliases: []string{"bootstraptoken"},
		Short:   "Generates bootstrap tokens used to join a node to a cluster",
		Example: bootstrapTokenExamples,
		Long:    bootstrapTokenLongDesc,
		InheritFlags: []string{
			options.CfgPath,
			options.KubeconfigPath,
			options.SkipTokenPrint,
		},
		Run: runBootstrapToken,
	}
```
###### 分别创建token与rbac
```
	fmt.Println("[bootstrap-token] Configuring bootstrap tokens, cluster-info ConfigMap, RBAC Roles")
	// Create the default node bootstrap token
	if err := nodebootstraptokenphase.UpdateOrCreateTokens(client, false, data.Cfg().BootstrapTokens); err != nil {
		return errors.Wrap(err, "error updating or creating token")
	}
	// Create RBAC rules that makes the bootstrap tokens able to get nodes
	if err := nodebootstraptokenphase.AllowBoostrapTokensToGetNodes(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to get Nodes")
	}
	// Create RBAC rules that makes the bootstrap tokens able to post CSRs
	if err := nodebootstraptokenphase.AllowBootstrapTokensToPostCSRs(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to post CSRs")
	}
	// Create RBAC rules that makes the bootstrap tokens able to get their CSRs approved automatically
	if err := nodebootstraptokenphase.AutoApproveNodeBootstrapTokens(client); err != nil {
		return errors.Wrap(err, "error auto-approving node bootstrap tokens")
	}

	// Create/update RBAC rules that makes the nodes to rotate certificates and get their CSRs approved automatically
	if err := nodebootstraptokenphase.AutoApproveNodeCertificateRotation(client); err != nil {
		return err
	}

	// Create the cluster-info ConfigMap with the associated RBAC rules
	if err := clusterinfophase.CreateBootstrapConfigMapIfNotExists(client, data.KubeConfigPath()); err != nil {
		return errors.Wrap(err, "error creating bootstrap ConfigMap")
	}
	if err := clusterinfophase.CreateClusterInfoRBACRules(client); err != nil {
		return errors.Wrap(err, "error creating clusterinfo RBAC rules")
	}
```
##### 12、在NewKubeletFinalizePhase函数里又定义了多个平面
```
return workflow.Phase{
		Name:    "kubelet-finalize",
		Short:   "Updates settings relevant to the kubelet after TLS bootstrap",
		Example: kubeletFinalizePhaseExample,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Run all kubelet-finalize phases",
				InheritFlags:   []string{options.CfgPath, options.CertificatesDir},
				Example:        kubeletFinalizePhaseExample,
				RunAllSiblings: true,
			},
			{
				Name:         "experimental-cert-rotation",
				Short:        "Enable kubelet client certificate rotation",
				InheritFlags: []string{options.CfgPath, options.CertificatesDir},
				Run:          runKubeletFinalizeCertRotation,
			},
		},
	}
```
###### 在runKubeletFinalizeCertRotation函数里执行了更新kubelet.conf的操作
```
//更新了client-certificate: /var/lib/kubelet/pki/kubelet-client-current.pem
//     client-key: /var/lib/kubelet/pki/kubelet-client-current.pem
// Update the client certificate and key of the node authorizer to point to the PEM symbolic link.
info.ClientKeyData = []byte{}
info.ClientCertificateData = []byte{}
info.ClientKey = pemPath
info.ClientCertificate = pemPath

// Writes the kubeconfig back to disk.
	if err = clientcmd.WriteToFile(*kubeconfig, kubeconfigPath); err != nil {
		return errors.Wrapf(err, "failed to serialize %q", kubeconfigPath)
	}

	// Restart the kubelet.
	klog.V(1).Info("[kubelet-finalize] Restarting the kubelet to enable client certificate rotation")
	kubeletphase.TryRestartKubelet()

```
##### 13、在NewAddonPhase函数里又定义了多个平面，主要是runCoreDNSAddon和runKubeProxyAddon
```
return workflow.Phase{
		Name:  "addon",
		Short: "Install required addons for passing conformance tests",
		Long:  cmdutil.MacroCommandLongDescription,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Install all the addons",
				InheritFlags:   getAddonPhaseFlags("all"),
				RunAllSiblings: true,
			},
			{
				Name:         "coredns",
				Short:        "Install the CoreDNS addon to a Kubernetes cluster",
				Long:         coreDNSAddonLongDesc,
				InheritFlags: getAddonPhaseFlags("coredns"),
				Run:          runCoreDNSAddon,
			},
			{
				Name:         "kube-proxy",
				Short:        "Install the kube-proxy addon to a Kubernetes cluster",
				Long:         kubeProxyAddonLongDesc,
				InheritFlags: getAddonPhaseFlags("kube-proxy"),
				Run:          runKubeProxyAddon,
			},
		},
	}
```
###### 通过代码发现CoreDNS为deployment、Proxy为daemonset，只是声明了这两个服务并没有写入到addon下成为插件的静态文件
```
// CoreDns创建了ConfigMap、ClusterRole、ClusterRoleBinding、ServiceAccount、Deployment、Service
deploymentsClient := client.AppsV1().Deployments(metav1.NamespaceSystem)
	deployments, err := deploymentsClient.List(context.TODO(), metav1.ListOptions{LabelSelector: "k8s-app=kube-dns"})
	if err != nil {
		return &replicas, errors.Wrap(err, "couldn't retrieve DNS addon deployments")
	}
	switch len(deployments.Items) {
	case 0:
		return &replicas, nil
	case 1:
		return deployments.Items[0].Spec.Replicas, nil
	default:
		return &replicas, errors.Errorf("multiple DNS addon deployments found: %v", deployments.Items)
	}

// Proxy创建了ConfigMap、Daemonset、ClusterRoleBinding、RoleBinding、Role
// CreateOrUpdateDaemonSet creates a DaemonSet if the target resource doesn't exist. If the resource exists already, this function will update the resource instead.
	if _, err := client.AppsV1().DaemonSets(ds.ObjectMeta.Namespace).Create(context.TODO(), ds, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "unable to create daemonset")
		}

		if _, err := client.AppsV1().DaemonSets(ds.ObjectMeta.Namespace).Update(context.TODO(), ds, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "unable to update daemonset")
		}
	}
	return nil
}
```

# TODO
```
回顾init生成的环境有几点疑问
  1、kubectl get secrets -A 发现非常多的证书，哪来的？
  2、kubectl get role -A
  3、kubectl get serviceaccout -A
  4、NewBootstrapTokenPhase平面里创建的bootstrap的token与rbac作用是干嘛的？

//这边的 serviceaccount、secrets、role 注意观察的话会发现都是以 controller 的名字为开头的
//观察过后发现 kube-controller-manager.yaml 里有 --use-service-account-credentials=true 这个参数
//查看参数代码后发现参数作用

//这边生成一个新的客户端，提示了如果这个参数，或者没有其他控制器创建 serviceacconut，后续会超时
if c.ComponentConfig.KubeCloudShared.UseServiceAccountCredentials {
		if len(c.ComponentConfig.SAController.ServiceAccountKeyFile) == 0 {
			// It's possible another controller process is creating the tokens for us.
			// If one isn't, we'll timeout and exit when our client builder is unable to create the tokens.
			klog.Warningf("--use-service-account-credentials was specified without providing a --service-account-private-key-file")
		}

		clientBuilder = clientbuilder.NewDynamicClientBuilder(
			restclient.AnonymousClientConfig(c.Kubeconfig),
			c.Client.CoreV1(),
			metav1.NamespaceSystem)
	} else {
		clientBuilder = rootClientBuilder
	}  

//定义一个serviceaccount控制器
saTokenControllerInitFunc := serviceAccountTokenControllerStarter{rootClientBuilder: rootClientBuilder}.startServiceAccountTokenController

run := func(ctx context.Context, startSATokenController InitFunc, initializersFunc ControllerInitializersFunc) 

//run 时会根据控制器名称初始化创建 serviceaccount
// No leader election, run directly
if !c.ComponentConfig.Generic.LeaderElection.LeaderElect {
	run(context.TODO(), saTokenControllerInitFunc, NewControllerInitializers)
	panic("unreachable")
}

```
