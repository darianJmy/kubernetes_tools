# NewUploadConfigPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewPreflighPhase平面主要工作创建configmap、role、rolebindings、node 增加注解

##### 1、NewUploadConfig 平面有 3 个子平面
```
return workflow.Phase{
		Name:    "upload-config",
		Aliases: []string{"uploadconfig"},
		Short:   "Upload the kubeadm and kubelet configuration to a ConfigMap",
		Long:    cmdutil.MacroCommandLongDescription,
		Phases: []workflow.Phase{
			{
				Name:           "all",
				Short:          "Upload all configuration to a config map",
				RunAllSiblings: true,
				InheritFlags:   getUploadConfigPhaseFlags(),
			},
			{
				Name:         "kubeadm",
				Short:        "Upload the kubeadm ClusterConfiguration to a ConfigMap",
				Long:         uploadKubeadmConfigLongDesc,
				Example:      uploadKubeadmConfigExample,
				Run:          runUploadKubeadmConfig,
				InheritFlags: getUploadConfigPhaseFlags(),
			},
			{
				Name:         "kubelet",
				Short:        "Upload the kubelet component config to a ConfigMap",
				Long:         uploadKubeletConfigLongDesc,
				Example:      uploadKubeletConfigExample,
				Run:          runUploadKubeletConfig,
				InheritFlags: getUploadConfigPhaseFlags(),
			},
		},
	}
```

##### 2、有一个平面名称为 all 作用就是执行所有平面

##### 3、runUploadKubeadmConfig 创建 configmap
```
// runUploadKubeadmConfig uploads the kubeadm configuration to a ConfigMap
func runUploadKubeadmConfig(c workflow.RunData) error {
    //这边是生成结构体、客户端
	cfg, client, err := getUploadConfigData(c)
	if err != nil {
		return err
	}

	klog.V(1).Infoln("[upload-config] Uploading the kubeadm ClusterConfiguration to a ConfigMap")
    //这边通过客户端上传结构体数据到 configmap
	if err := uploadconfig.UploadConfiguration(cfg, client); err != nil {
		return errors.Wrap(err, "error uploading the kubeadm ClusterConfiguration")
	}
	return nil
}


// UploadConfiguration saves the InitConfiguration used for later reference (when upgrading for instance)
func UploadConfiguration(cfg *kubeadmapi.InitConfiguration, client clientset.Interface) error {
	fmt.Printf("[upload-config] Storing the configuration used in ConfigMap %q in the %q Namespace\n", kubeadmconstants.KubeadmConfigConfigMap, metav1.NamespaceSystem)

	// Prepare the ClusterConfiguration for upload
	// The components store their config in their own ConfigMaps, then reset the .ComponentConfig struct;
	// We don't want to mutate the cfg itself, so create a copy of it using .DeepCopy of it first
    //new 了一个结构体给 clusterConfigurationToUpload，这边相当于定义格式
	clusterConfigurationToUpload := cfg.ClusterConfiguration.DeepCopy()
	clusterConfigurationToUpload.ComponentConfigs = kubeadmapi.ComponentConfigMap{}

	// Marshal the ClusterConfiguration into YAML
    //这边的是实例化 cfg 结构体，传到 MarshalKubeadmConfigObject 函数生成字节
	clusterConfigurationYaml, err := configutil.MarshalKubeadmConfigObject(clusterConfigurationToUpload)
	if err != nil {
		return err
	}
    //这边是创建 configmap
	err = apiclient.CreateOrMutateConfigMap(client, &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmconstants.KubeadmConfigConfigMap,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			kubeadmconstants.ClusterConfigurationConfigMapKey: string(clusterConfigurationYaml),
		},
	}, func(cm *v1.ConfigMap) error {
		// Upgrade will call to UploadConfiguration with a modified KubernetesVersion reflecting the new
		// Kubernetes version. In that case, the mutation path will take place.
		cm.Data[kubeadmconstants.ClusterConfigurationConfigMapKey] = string(clusterConfigurationYaml)
		return nil
	})
	if err != nil {
		return err
	}

	// Ensure that the NodesKubeadmConfigClusterRoleName exists
    //创建名字为 kubeadm:nodes-kubeadm-config 的 Role ，权限 get、资源 configmaps、资源名 kubeadm-config
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
	if err != nil {
		return err
	}

	// Binds the NodesKubeadmConfigClusterRoleName to all the bootstrap tokens
	// that are members of the system:bootstrappers:kubeadm:default-node-token group
	// and to all nodes
    //创建名字为 kubeadm:nodes-kubeadm-config 的 RoleBindings
	return apiclient.CreateOrUpdateRoleBinding(client, &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NodesKubeadmConfigClusterRoleName,
			Namespace: metav1.NamespaceSystem,
		},
		RoleRef: rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "Role",
			Name:     NodesKubeadmConfigClusterRoleName,
		},
		Subjects: []rbac.Subject{
			{
				Kind: rbac.GroupKind,
				Name: kubeadmconstants.NodeBootstrapTokenAuthGroup,
			},
			{
				Kind: rbac.GroupKind,
				Name: kubeadmconstants.NodesGroup,
			},
		},
	})
}
```

##### 4、runUploadKubeletConfig 创建 configmap
```
//这个平面跟生成 kubeadm configmap 类似
//创建 kubelet-config configmap 、kubeadm:kubelet-config Role、kubeadm:kubelet-config RoleBindings
//但是这边需要判断一下kubelet版本，如果使用的老版本 name 后面会跟上后缀 kubelet-config-1.18
cfg, client, err := getUploadConfigData(c)
	if err != nil {
		return err
	}

	klog.V(1).Infoln("[upload-config] Uploading the kubelet component config to a ConfigMap")
	if err = kubeletphase.CreateConfigMap(&cfg.ClusterConfiguration, client); err != nil {
		return errors.Wrap(err, "error creating kubelet configuration ConfigMap")
	}

	klog.V(1).Infoln("[upload-config] Preserving the CRISocket information for the control-plane node")
    //这边有个 patchnodephase 给 node 节点增加注解 kubeadm.alpha.kubernetes.io/cri-socket = criSocket
	if err := patchnodephase.AnnotateCRISocket(client, cfg.NodeRegistration.Name, cfg.NodeRegistration.CRISocket); err != nil {
		return errors.Wrap(err, "Error writing Crisocket information for the control-plane node")
	}
	return nil 
```