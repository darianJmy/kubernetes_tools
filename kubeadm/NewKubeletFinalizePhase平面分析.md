# NewKubeletFinalizePhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewKubeletFinalizePhase 平面主要工作是更新 /etc/kubernetes/kubelet.conf

##### 1、NewKubeletFinalize 不涉及子平面
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

##### 2、runKubeletFinalizeCertRotation 函数给 /etc/kubernetes/kubelet.conf 添加配置，重启kubelet
```
// runKubeletFinalizeCertRotation detects if the kubelet certificate rotation is enabled
// and updates the kubelet.conf file to point to a rotatable certificate and key for the
// Node user.
func runKubeletFinalizeCertRotation(c workflow.RunData) error {
	data, ok := c.(InitData)
	if !ok {
		return errors.New("kubelet-finalize phase invoked with an invalid data struct")
	}

	// Check if the user has added the kubelet --cert-dir flag.
	// If yes, use that path, else use the kubeadm provided value.
	cfg := data.Cfg()
	pkiPath := filepath.Join(data.KubeletDir(), "pki")
	val, ok := cfg.NodeRegistration.KubeletExtraArgs["cert-dir"]
	if ok {
		pkiPath = val
	}
    //检测有没有 kubelet-client-current.pem 这个文件
	// Check for the existence of the kubelet-client-current.pem file in the kubelet certificate directory.
	rotate := false
	pemPath := filepath.Join(pkiPath, "kubelet-client-current.pem")
	if _, err := os.Stat(pemPath); err == nil {
		klog.V(1).Infof("[kubelet-finalize] Assuming that kubelet client certificate rotation is enabled: found %q", pemPath)
		rotate = true
	} else {
		klog.V(1).Infof("[kubelet-finalize] Assuming that kubelet client certificate rotation is disabled: %v", err)
	}

	// Exit early if rotation is disabled.
	if !rotate {
		return nil
	}

	kubeconfigPath := filepath.Join(kubeadmconstants.KubernetesDir, kubeadmconstants.KubeletKubeConfigFileName)
	fmt.Printf("[kubelet-finalize] Updating %q to point to a rotatable kubelet client certificate and key\n", kubeconfigPath)

	// Exit early if dry-running is enabled.
	if data.DryRun() {
		return nil
	}

	// Load the kubeconfig from disk.
    // 加载原始的 kubelet.conf
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return errors.Wrapf(err, "could not load %q", kubeconfigPath)
	}

	// Perform basic validation. The errors here can only happen if the kubelet.conf was corrupted.
	userName := fmt.Sprintf("%s%s", kubeadmconstants.NodesUserPrefix, cfg.NodeRegistration.Name)
	info, ok := kubeconfig.AuthInfos[userName]
	if !ok {
		return errors.Errorf("the file %q does not contain authentication for user %q", kubeconfigPath, cfg.NodeRegistration.Name)
	}

	// Update the client certificate and key of the node authorizer to point to the PEM symbolic link.
	info.ClientKeyData = []byte{}
	info.ClientCertificateData = []byte{}
	info.ClientKey = pemPath
	info.ClientCertificate = pemPath

	// Writes the kubeconfig back to disk.
    //这边对于 kubelet.conf 操作文件新增了 users 
	if err = clientcmd.WriteToFile(*kubeconfig, kubeconfigPath); err != nil {
		return errors.Wrapf(err, "failed to serialize %q", kubeconfigPath)
	}

	// Restart the kubelet.
    //重启
	klog.V(1).Info("[kubelet-finalize] Restarting the kubelet to enable client certificate rotation")
	kubeletphase.TryRestartKubelet()

	return nil
}
```