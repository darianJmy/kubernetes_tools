# NewUploadCertsPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewUploadCertsPhase平面主要工作是更新证书

##### 1、NewUploadCerts 平面没有子平面
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
```

##### 2、runUploadCerts 平面更新证书
```
func runUploadCerts(c workflow.RunData) error {
	data, ok := c.(InitData)
	if !ok {
		return errors.New("upload-certs phase invoked with an invalid data struct")
	}
    //但是如果是新环境是不需要的，也不需要加参数所以默认不执行
	if !data.UploadCerts() {
		fmt.Printf("[upload-certs] Skipping phase. Please see --%s\n", options.UploadCerts)
		return nil
	}
    //如果要执行，new 一个客户端
	client, err := data.Client()
	if err != nil {
		return err
	}

    //如果不定 key，则自己创建一个 key
	if len(data.CertificateKey()) == 0 {
		certificateKey, err := copycerts.CreateCertificateKey()
		if err != nil {
			return err
		}
		data.SetCertificateKey(certificateKey)
	}
    //如果添加参数执行 updatecerts，这边为uploadcerts
    //UploadCerts 函数为通过 key 生成一个 token，把 token 给到 kubeadm-certs secret中，添加 role 权限
	if err := copycerts.UploadCerts(client, data.Cfg(), data.CertificateKey()); err != nil {
		return errors.Wrap(err, "error uploading certs")
	}
	if !data.SkipCertificateKeyPrint() {
		fmt.Printf("[upload-certs] Using certificate key:\n%s\n", data.CertificateKey())
	}
	return nil
}


//UploadCerts save certs needs to join a new control-plane on kubeadm-certs sercret.
func UploadCerts(client clientset.Interface, cfg *kubeadmapi.InitConfiguration, key string) error {
	fmt.Printf("[upload-certs] Storing the certificates in Secret %q in the %q Namespace\n", kubeadmconstants.KubeadmCertsSecret, metav1.NamespaceSystem)
	decodedKey, err := hex.DecodeString(key)
	if err != nil {
		return errors.Wrap(err, "error decoding certificate key")
	}
	tokenID, err := createShortLivedBootstrapToken(client)
	if err != nil {
		return err
	}

	secretData, err := getDataFromDisk(cfg, decodedKey)
	if err != nil {
		return err
	}
	ref, err := getSecretOwnerRef(client, tokenID)
	if err != nil {
		return err
	}

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
}

```