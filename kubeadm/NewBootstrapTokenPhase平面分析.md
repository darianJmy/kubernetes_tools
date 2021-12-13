# NewBootstrapTokenPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewBootstrapTokenPhase平面主要工作时创建 configmap、role、rolebindings

##### 1、NewBootstrapToken 不涉及子平面
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

##### 2、runBootstrapToken 函数
```
func runBootstrapToken(c workflow.RunData) error {
	data, ok := c.(InitData)
	if !ok {
		return errors.New("bootstrap-token phase invoked with an invalid data struct")
	}
    //new 一个 client
	client, err := data.Client()
	if err != nil {
		return err
	}

	if !data.SkipTokenPrint() {
        //data.Tokens里面又个String函数，把 BootstrapTokenString 结构体 ID、Secret 输出
		tokens := data.Tokens()
		if len(tokens) == 1 {
			fmt.Printf("[bootstrap-token] Using token: %s\n", tokens[0])
		} else if len(tokens) > 1 {
			fmt.Printf("[bootstrap-token] Using tokens: %v\n", tokens)
		}
	}

	fmt.Println("[bootstrap-token] Configuring bootstrap tokens, cluster-info ConfigMap, RBAC Roles")
	// Create the default node bootstrap token
    //new 一个 client get 获取 secretname 如果不等于空或者有错误，就报错如果没有就执行创建 secret 任务，创建的 secret 为 bootstrap-token- + tokens结构体 id，数据为结构体内容
	if err := nodebootstraptokenphase.UpdateOrCreateTokens(client, false, data.Cfg().BootstrapTokens); err != nil {
		return errors.Wrap(err, "error updating or creating token")
	}
	// Create RBAC rules that makes the bootstrap tokens able to get nodes
    //创建 rbac kubeadm:get-nodes
	if err := nodebootstraptokenphase.AllowBoostrapTokensToGetNodes(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to get Nodes")
	}
    //创建 rbac kubeadm:kubelet-bootstrap
	// Create RBAC rules that makes the bootstrap tokens able to post CSRs
	if err := nodebootstraptokenphase.AllowBootstrapTokensToPostCSRs(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to post CSRs")
	}
    //创建 rbac kubeadm:node-autoapprove-bootstrap
	// Create RBAC rules that makes the bootstrap tokens able to get their CSRs approved automatically
	if err := nodebootstraptokenphase.AutoApproveNodeBootstrapTokens(client); err != nil {
		return errors.Wrap(err, "error auto-approving node bootstrap tokens")
	}

    //创建 rbac kubeadm:node-autoapprove-certificate-rotation
	// Create/update RBAC rules that makes the nodes to rotate certificates and get their CSRs approved automatically
	if err := nodebootstraptokenphase.AutoApproveNodeCertificateRotation(client); err != nil {
		return err
	}

    //创建 configmap cluster-info 在 kube-public namespaces
	// Create the cluster-info ConfigMap with the associated RBAC rules
	if err := clusterinfophase.CreateBootstrapConfigMapIfNotExists(client, data.KubeConfigPath()); err != nil {
		return errors.Wrap(err, "error creating bootstrap ConfigMap")
	}
    //创建 rbac kubeadm:bootstrap-signer-clusterinfo 在 kube-public namespaces
	if err := clusterinfophase.CreateClusterInfoRBACRules(client); err != nil {
		return errors.Wrap(err, "error creating clusterinfo RBAC rules")
	}
	return nil
}
```
