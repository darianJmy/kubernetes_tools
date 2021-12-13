# NewAddonPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewAddonPhase平面主要工作创建插件 coredns、kube-proxy

##### 1、NewAddonPhase 有 3 个平面， all 平面执行所有平面
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

##### 2、coredns 平面创建 coredns deployment
```
// runCoreDNSAddon installs CoreDNS addon to a Kubernetes cluster
func runCoreDNSAddon(c workflow.RunData) error {
    //new 一个 client
	cfg, client, err := getInitData(c)
	if err != nil {
		return err
	}
    //判断是否有dns，如果有但是数量不对，就replicas 如果没有就创一个 deployment，list 的数据是通过标签获取的 k8s-app=kube-dns
	return dnsaddon.EnsureDNSAddon(&cfg.ClusterConfiguration, client)
}

// EnsureDNSAddon creates the CoreDNS addon
func EnsureDNSAddon(cfg *kubeadmapi.ClusterConfiguration, client clientset.Interface) error {
    //这边通过客户端 list 获取 deployment 里面 dns，判断数量switch Item 0、1、default
	replicas, err := deployedDNSReplicas(client, coreDNSReplicas)
	if err != nil {
		return err
	}
    //执行创建任务
	return coreDNSAddon(cfg, client, replicas)
}

func coreDNSAddon(cfg *kubeadmapi.ClusterConfiguration, client clientset.Interface, replicas *int32) error {
	// Get the YAML manifest
	coreDNSDeploymentBytes, err := kubeadmutil.ParseTemplate(CoreDNSDeployment, struct {
		DeploymentName, Image, OldControlPlaneTaintKey, ControlPlaneTaintKey string
		Replicas                                                             *int32
	}{
		DeploymentName: kubeadmconstants.CoreDNSDeploymentName,
		Image:          images.GetDNSImage(cfg),
		// TODO: https://github.com/kubernetes/kubeadm/issues/2200
		OldControlPlaneTaintKey: kubeadmconstants.LabelNodeRoleOldControlPlane,
		ControlPlaneTaintKey:    kubeadmconstants.LabelNodeRoleControlPlane,
		Replicas:                replicas,
	})
	if err != nil {
		return errors.Wrap(err, "error when parsing CoreDNS deployment template")
	}

	// Get the config file for CoreDNS
	coreDNSConfigMapBytes, err := kubeadmutil.ParseTemplate(CoreDNSConfigMap, struct{ DNSDomain, UpstreamNameserver, StubDomain string }{
		DNSDomain: cfg.Networking.DNSDomain,
	})
	if err != nil {
		return errors.Wrap(err, "error when parsing CoreDNS configMap template")
	}

	dnsip, err := kubeadmconstants.GetDNSIP(cfg.Networking.ServiceSubnet)
	if err != nil {
		return err
	}

	coreDNSServiceBytes, err := kubeadmutil.ParseTemplate(CoreDNSService, struct{ DNSIP string }{
		DNSIP: dnsip.String(),
	})

	if err != nil {
		return errors.Wrap(err, "error when parsing CoreDNS service template")
	}
    //创建 coredns
	if err := createCoreDNSAddon(coreDNSDeploymentBytes, coreDNSServiceBytes, coreDNSConfigMapBytes, client); err != nil {
		return err
	}
	fmt.Println("[addons] Applied essential addon: CoreDNS")
	return nil
}
```

##### 2、kube-proxy 平面创建 kube-proxy serviceaccount、 kube-proxy configmap、kube-proxy daemonset、kube-proxy rbac
```
// EnsureProxyAddon creates the kube-proxy addons
func EnsureProxyAddon(cfg *kubeadmapi.ClusterConfiguration, localEndpoint *kubeadmapi.APIEndpoint, client clientset.Interface) error {
	if err := CreateServiceAccount(client); err != nil {
		return errors.Wrap(err, "error when creating kube-proxy service account")
	}

	if err := createKubeProxyConfigMap(cfg, localEndpoint, client); err != nil {
		return err
	}

	if err := createKubeProxyAddon(cfg, client); err != nil {
		return err
	}

	if err := CreateRBACRules(client); err != nil {
		return errors.Wrap(err, "error when creating kube-proxy RBAC rules")
	}

	fmt.Println("[addons] Applied essential addon: kube-proxy")
	return nil
}

```