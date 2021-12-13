# NewControlPlanePhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewControlPlanePhase 平面主要创建 api-server、controller、scheduler 静态 pod

##### 1、ControlPlane 平面涉及子平面，runControlPlanePhase 函数负责 Print 提示，主要创建任务还是在子平面里
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
	return phase
```

##### 2、runControlPlanePhase 函数会 print 一些数据作为创建文件的提示，执行 ControlPlane 平面内容后会执行子平面内容
```
	fmt.Printf("[control-plane] Using manifest folder %q\n", data.ManifestDir())
	return nil
```

##### 3、newControlPlaneSubphase 平面里有一个结构体里面有 Run 变量为 runControlPlaneSubphase 函数，这个函数根据传进去的值来创建静态 pod 文件
```
//compoent 为 string 所以这边根据 名字来创建静态 pod 文件
func newControlPlaneSubphase(component string) workflow.Phase {
	phase := workflow.Phase{
		Name:         controlPlanePhaseProperties[component].name,
		Short:        controlPlanePhaseProperties[component].short,
		Run:          runControlPlaneSubphase(component),
		InheritFlags: getControlPlanePhaseFlags(component),
	}
	return phase
}

func runControlPlaneSubphase(component string) func(c workflow.RunData) error {
	return func(c workflow.RunData) error {
		data, ok := c.(InitData)
		if !ok {
			return errors.New("control-plane phase invoked with an invalid data struct")
		}
		cfg := data.Cfg()
        //输出提示
		fmt.Printf("[control-plane] Creating static Pod manifest for %q\n", component)
        //创建静态 pod 函数
		return controlplane.CreateStaticPodFiles(data.ManifestDir(), data.PatchesDir(), &cfg.ClusterConfiguration, &cfg.LocalAPIEndpoint, data.DryRun(), component)
	}
}

// CreateStaticPodFiles creates all the requested static pod files.
func CreateStaticPodFiles(manifestDir, patchesDir string, cfg *kubeadmapi.ClusterConfiguration, endpoint *kubeadmapi.APIEndpoint, isDryRun bool, componentNames ...string) error {
	// gets the StaticPodSpecs, actualized for the current ClusterConfiguration
    //log 打印
	klog.V(1).Infoln("[control-plane] getting StaticPodSpecs")
    //specs 是获取的一个实例化结构体，GetStaticPodSpecs 结构体里有 api-server、kube-controller-manager、kube-scheduler
    //结构体数据有：Name、Image、ImagePullPolicy、Command、VolumeMounts、LivenessProbe、ReadinessProbe、StartupProbe、Resources、Env
	specs := GetStaticPodSpecs(cfg, endpoint)

	var usersAndGroups *users.UsersAndGroups
	var err error
	if features.Enabled(cfg.FeatureGates, features.RootlessControlPlane) {
		if isDryRun {
			fmt.Printf("[dryrun] Would create users and groups for %+v to run as non-root\n", componentNames)
		} else {
            //
			usersAndGroups, err = staticpodutil.GetUsersAndGroups()
			if err != nil {
				return errors.Wrap(err, "failed to create users and groups")
			}
		}
	}

	// creates required static pod specs
	for _, componentName := range componentNames {
		// retrieves the StaticPodSpec for given component
        //获取指定的静态二进制文件结构题
		spec, exists := specs[componentName]
		if !exists {
			return errors.Errorf("couldn't retrieve StaticPodSpec for %q", componentName)
		}

		// print all volumes that are mounted
		for _, v := range spec.Spec.Volumes {
			klog.V(2).Infof("[control-plane] adding volume %q for component %q", v.Name, componentName)
		}

		if features.Enabled(cfg.FeatureGates, features.RootlessControlPlane) {
			if isDryRun {
				fmt.Printf("[dryrun] Would update static pod manifest for %q to run run as non-root\n", componentName)
			} else {
                //这边不大明白，因为返回值为nil，nil
				if usersAndGroups != nil {
					if err := staticpodutil.RunComponentAsNonRoot(componentName, &spec, usersAndGroups, cfg); err != nil {
						return errors.Wrapf(err, "failed to run component %q as non-root", componentName)
					}
				}
			}
		}

		// if patchesDir is defined, patch the static Pod manifest
        //判断初始化是有没有指定 k8s 数据存放路径，如果没有指定则不执行
		if patchesDir != "" {
			patchedSpec, err := staticpodutil.PatchStaticPod(&spec, patchesDir, os.Stdout)
			if err != nil {
				return errors.Wrapf(err, "failed to patch static Pod manifest file for %q", componentName)
			}
			spec = *patchedSpec
		}

		// writes the StaticPodSpec to disk
        //这边为写入数据，先创建文件，再写入文件
		if err := staticpodutil.WriteStaticPodToDisk(componentName, manifestDir, spec); err != nil {
			return errors.Wrapf(err, "failed to create static pod manifest file for %q", componentName)
		}

		klog.V(1).Infof("[control-plane] wrote static Pod manifest for component %q to %q\n", componentName, kubeadmconstants.GetStaticPodFilepath(componentName, manifestDir))
	}

	return nil
}
```

##### 4、这边最关心的是静态文件里面的内容如何生成的
```
specs := GetStaticPodSpecs(cfg, endpoint)

// GetStaticPodSpecs returns all staticPodSpecs actualized to the context of the current configuration
// NB. this methods holds the information about how kubeadm creates static pod manifests.
func GetStaticPodSpecs(cfg *kubeadmapi.ClusterConfiguration, endpoint *kubeadmapi.APIEndpoint) map[string]v1.Pod {
	// Get the required hostpath mounts
	mounts := getHostPathVolumesForTheControlPlane(cfg)

	// Prepare static pod specs
	staticPodSpecs := map[string]v1.Pod{
        //staticpodutil.ComponentPod 是一个函数，主要是把数据放到静态 pod 结构体中
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
		kubeadmconstants.KubeControllerManager: staticpodutil.ComponentPod(v1.Container{
			Name:            kubeadmconstants.KubeControllerManager,
			Image:           images.GetKubernetesImage(kubeadmconstants.KubeControllerManager, cfg),
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         getControllerManagerCommand(cfg),
			VolumeMounts:    staticpodutil.VolumeMountMapToSlice(mounts.GetVolumeMounts(kubeadmconstants.KubeControllerManager)),
			LivenessProbe:   staticpodutil.LivenessProbe(staticpodutil.GetControllerManagerProbeAddress(cfg), "/healthz", kubeadmconstants.KubeControllerManagerPort, v1.URISchemeHTTPS),
			StartupProbe:    staticpodutil.StartupProbe(staticpodutil.GetControllerManagerProbeAddress(cfg), "/healthz", kubeadmconstants.KubeControllerManagerPort, v1.URISchemeHTTPS, cfg.APIServer.TimeoutForControlPlane),
			Resources:       staticpodutil.ComponentResources("200m"),
			Env:             kubeadmutil.GetProxyEnvVars(),
		}, mounts.GetVolumes(kubeadmconstants.KubeControllerManager), nil),
		kubeadmconstants.KubeScheduler: staticpodutil.ComponentPod(v1.Container{
			Name:            kubeadmconstants.KubeScheduler,
			Image:           images.GetKubernetesImage(kubeadmconstants.KubeScheduler, cfg),
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         getSchedulerCommand(cfg),
			VolumeMounts:    staticpodutil.VolumeMountMapToSlice(mounts.GetVolumeMounts(kubeadmconstants.KubeScheduler)),
			LivenessProbe:   staticpodutil.LivenessProbe(staticpodutil.GetSchedulerProbeAddress(cfg), "/healthz", kubeadmconstants.KubeSchedulerPort, v1.URISchemeHTTPS),
			StartupProbe:    staticpodutil.StartupProbe(staticpodutil.GetSchedulerProbeAddress(cfg), "/healthz", kubeadmconstants.KubeSchedulerPort, v1.URISchemeHTTPS, cfg.APIServer.TimeoutForControlPlane),
			Resources:       staticpodutil.ComponentResources("100m"),
			Env:             kubeadmutil.GetProxyEnvVars(),
		}, mounts.GetVolumes(kubeadmconstants.KubeScheduler), nil),
	}
	return staticPodSpecs
}


// ComponentPod returns a Pod object from the container, volume and annotations specifications
func ComponentPod(container v1.Container, volumes map[string]v1.Volume, annotations map[string]string) v1.Pod {
	return v1.Pod{
        //TypeMeta 与 ObjectMeta 没什么变化，主要是 Spec 里 Containers 数组
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      container.Name,
			Namespace: metav1.NamespaceSystem,
			// The component and tier labels are useful for quickly identifying the control plane Pods when doing a .List()
			// against Pods in the kube-system namespace. Can for example be used together with the WaitForPodsWithLabel function
			Labels:      map[string]string{"component": container.Name, "tier": kubeadmconstants.ControlPlaneTier},
			Annotations: annotations,
		},
		Spec: v1.PodSpec{

            //这边的Containers就是传进来的结构体，里面有个 command 函数，对应着执行的参数
			Containers:        []v1.Container{container},
			PriorityClassName: "system-node-critical",
			HostNetwork:       true,
			Volumes:           VolumeMapToSlice(volumes),
			SecurityContext: &v1.PodSecurityContext{
				SeccompProfile: &v1.SeccompProfile{
					Type: v1.SeccompProfileTypeRuntimeDefault,
				},
			},
		},
	}
}

/这边不仅对应了自身的api-server ip 、service-account、而且定义了etcd
// getAPIServerCommand builds the right API server command from the given config object and version
func getAPIServerCommand(cfg *kubeadmapi.ClusterConfiguration, localAPIEndpoint *kubeadmapi.APIEndpoint) []string {
	defaultArguments := map[string]string{
		"advertise-address":                localAPIEndpoint.AdvertiseAddress,
		"enable-admission-plugins":         "NodeRestriction",
		"service-cluster-ip-range":         cfg.Networking.ServiceSubnet,
		"service-account-key-file":         filepath.Join(cfg.CertificatesDir, kubeadmconstants.ServiceAccountPublicKeyName),
		"service-account-signing-key-file": filepath.Join(cfg.CertificatesDir, kubeadmconstants.ServiceAccountPrivateKeyName),
		"service-account-issuer":           fmt.Sprintf("https://kubernetes.default.svc.%s", cfg.Networking.DNSDomain),
		"client-ca-file":                   filepath.Join(cfg.CertificatesDir, kubeadmconstants.CACertName),
		"tls-cert-file":                    filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerCertName),
		"tls-private-key-file":             filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerKeyName),
		"kubelet-client-certificate":       filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerKubeletClientCertName),
		"kubelet-client-key":               filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerKubeletClientKeyName),
		"enable-bootstrap-token-auth":      "true",
		"secure-port":                      fmt.Sprintf("%d", localAPIEndpoint.BindPort),
		"allow-privileged":                 "true",
		"kubelet-preferred-address-types":  "InternalIP,ExternalIP,Hostname",
		// add options to configure the front proxy.  Without the generated client cert, this will never be useable
		// so add it unconditionally with recommended values
		"requestheader-username-headers":     "X-Remote-User",
		"requestheader-group-headers":        "X-Remote-Group",
		"requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"requestheader-client-ca-file":       filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyCACertName),
		"requestheader-allowed-names":        "front-proxy-client",
		"proxy-client-cert-file":             filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyClientCertName),
		"proxy-client-key-file":              filepath.Join(cfg.CertificatesDir, kubeadmconstants.FrontProxyClientKeyName),
	}

	command := []string{"kube-apiserver"}

	// If the user set endpoints for an external etcd cluster
	if cfg.Etcd.External != nil {
		defaultArguments["etcd-servers"] = strings.Join(cfg.Etcd.External.Endpoints, ",")

		// Use any user supplied etcd certificates
		if cfg.Etcd.External.CAFile != "" {
			defaultArguments["etcd-cafile"] = cfg.Etcd.External.CAFile
		}
		if cfg.Etcd.External.CertFile != "" && cfg.Etcd.External.KeyFile != "" {
			defaultArguments["etcd-certfile"] = cfg.Etcd.External.CertFile
			defaultArguments["etcd-keyfile"] = cfg.Etcd.External.KeyFile
		}
	} else {
		// Default to etcd static pod on localhost
		// localhost IP family should be the same that the AdvertiseAddress
		etcdLocalhostAddress := "127.0.0.1"
		if utilsnet.IsIPv6String(localAPIEndpoint.AdvertiseAddress) {
			etcdLocalhostAddress = "::1"
		}
		defaultArguments["etcd-servers"] = fmt.Sprintf("https://%s", net.JoinHostPort(etcdLocalhostAddress, strconv.Itoa(kubeadmconstants.EtcdListenClientPort)))
		defaultArguments["etcd-cafile"] = filepath.Join(cfg.CertificatesDir, kubeadmconstants.EtcdCACertName)
		defaultArguments["etcd-certfile"] = filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerEtcdClientCertName)
		defaultArguments["etcd-keyfile"] = filepath.Join(cfg.CertificatesDir, kubeadmconstants.APIServerEtcdClientKeyName)

		// Apply user configurations for local etcd
		if cfg.Etcd.Local != nil {
			if value, ok := cfg.Etcd.Local.ExtraArgs["advertise-client-urls"]; ok {
				defaultArguments["etcd-servers"] = value
			}
		}
	}

	if cfg.APIServer.ExtraArgs == nil {
		cfg.APIServer.ExtraArgs = map[string]string{}
	}
	cfg.APIServer.ExtraArgs["authorization-mode"] = getAuthzModes(cfg.APIServer.ExtraArgs["authorization-mode"])
	command = append(command, kubeadmutil.BuildArgumentListFromMap(defaultArguments, cfg.APIServer.ExtraArgs)...)

	return command
}

```