# NewCertsPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go 建议先看看cobra
##### NewKubeConfigPhase平面主要工作是通过证书文件创建带上下文的启动配置文件

##### 1、KubeConfig 平面涉及子平面，runKubeConfig 函数负责 Print 提示，主要创建任务还是在子平面里
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
            //这边常量主要为admin.conf、kubelet.conf、controller-manager.conf、scheduler.conf
			NewKubeConfigFilePhase(kubeadmconstants.AdminKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.KubeletKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.ControllerManagerKubeConfigFileName),
			NewKubeConfigFilePhase(kubeadmconstants.SchedulerKubeConfigFileName),
		},
		Run: runKubeConfig,
	}
```

##### 2、runKubeConfig 函数会 print 一些数据作为创建文件的提示，执行kubeconfig平面内容后会执行子平面内容
```
fmt.Printf("[kubeconfig] Using kubeconfig folder %q\n", data.KubeConfigDir())
	return nil
```

##### 3、Phases 平面里有一个结构体里面 Name: all,  逗号后面跟的 NewKubeConfigFilePhase 函数，这个函数里面的 AdminKubeConfigFileName 为常量，大体就是这个函数生成了一个结构体
```
//所有结构体都是指向 runKubeConfigFile 函数
// NewKubeConfigFilePhase creates a kubeadm workflow phase that creates a kubeconfig file.
func NewKubeConfigFilePhase(kubeConfigFileName string) workflow.Phase {
	return workflow.Phase{
		Name:         kubeconfigFilePhaseProperties[kubeConfigFileName].name,
		Short:        kubeconfigFilePhaseProperties[kubeConfigFileName].short,
		Long:         fmt.Sprintf(kubeconfigFilePhaseProperties[kubeConfigFileName].long, kubeConfigFileName),
		Run:          runKubeConfigFile(kubeConfigFileName),
		InheritFlags: getKubeConfigPhaseFlags(kubeConfigFileName),
	}
}
```

##### 4、runKubeConfigFile 函数会 检查有没有外部 CA，如果有则跳过创建步骤，如果没有则创建新的配置文件
```
// if external CA mode, skip certificate authority generation
		if data.ExternalCA() {
            //输出一个提示
			fmt.Printf("[kubeconfig] External CA mode: Using user provided %s\n", kubeConfigFileName)
			// If using an external CA while dryrun, copy kubeconfig files to dryrun dir for later use
            //判断有没有 --dry-run 参数，如果有写入 /etc/kubernetes/ 下 .config 文件内容到 KubeConfigDir 下
            //--dry-run Don't apply any changes; just output what would be done.
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

//检测到没有外部 CA，没有 DryRun，执行创建命令
// if dryrunning, reads certificates from a temporary folder (and defer restore to the path originally specified by the user)
		cfg := data.Cfg()
		cfg.CertificatesDir = data.CertificateWriteDir()
		defer func() { cfg.CertificatesDir = data.CertificateDir() }()

// creates the KubeConfig file (or use existing)
        return kubeconfigphase.CreateKubeConfigFile(kubeConfigFileName, data.KubeConfigDir(), data.Cfg())
```

##### 5、CreateKubeConfigFile 函数传了3个值进去，kubeConfigFileName 为 .config 文件，data.KubeConfigDir() 为 initData 结构体里 kubeconfigDir， data.Cfg() 为 initData 结构体里 cfg， cfg 为 InitConfiguration 结构体，就是 init 是初始化的数据
```
//返回的是createKubeConfigFiles函数
// CreateKubeConfigFile creates a kubeconfig file.
// If the kubeconfig file already exists, it is used only if evaluated equal; otherwise an error is returned.
func CreateKubeConfigFile(kubeConfigFileName string, outDir string, cfg *kubeadmapi.InitConfiguration) error {
	klog.V(1).Infof("creating kubeconfig file for %s", kubeConfigFileName)
	return createKubeConfigFiles(outDir, cfg, kubeConfigFileName)
}
```

##### 6、createKubeConfigFiles 函数
```
// createKubeConfigFiles creates all the requested kubeconfig files.
// If kubeconfig files already exists, they are used only if evaluated equal; otherwise an error is returned.
func createKubeConfigFiles(outDir string, cfg *kubeadmapi.InitConfiguration, kubeConfigFileNames ...string) error {

	// gets the KubeConfigSpecs, actualized for the current InitConfiguration
    /这边specs等于一个结构体
	specs, err := getKubeConfigSpecs(cfg)
	if err != nil {
		return err
	}

    //kubeConfigFileNames 是一个数组，但是它里面一次只存一个数据
	for _, kubeConfigFileName := range kubeConfigFileNames {
		// retrieves the KubeConfigSpec for given kubeConfigFileName
        //这边 specs 是一个 map，spec 指定了 map 里面的一个数据
		spec, exists := specs[kubeConfigFileName]
		if !exists {
			return errors.Errorf("couldn't retrieve KubeConfigSpec for %s", kubeConfigFileName)
		}

		// builds the KubeConfig object
        这边是补全需要输入到 .config文件中的结构体
		config, err := buildKubeConfigFromSpec(spec, cfg.ClusterName, nil)
		if err != nil {
			return err
		}

        //createKubeConfigFileIfNotExists 函数里面 kubeconfigutil.WriteToDisk(kubeConfigFilePath, config) 
        //执行 WriteToDisk 函数，在函数里还有一个 validateKubeConfig(outDir, filename, config) 函数，
        //目的是读本地是否有这个文件
        //WriteToDisk 为配置具体内容写入文件，通过os.stat查看本地是否有这个文件，如果没有报错测不执行创建，并且输出文件已存在
		// writes the kubeconfig to disk if it does not exist
		if err = createKubeConfigFileIfNotExists(outDir, kubeConfigFileName, config); err != nil {
			return err
		}
	}

	return nil
}

if _, err := os.Stat(kubeConfigFilePath); err != nil {
		return err
	}
    
// WriteToFile serializes the config to yaml and writes it out to a file.  If not present, it creates the file with the mode 0600.  If it is present
// it stomps the contents
func WriteToFile(config clientcmdapi.Config, filename string) error {
	content, err := Write(config)
	if err != nil {
		return err
	}
	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	if err := ioutil.WriteFile(filename, content, 0600); err != nil {
		return err
	}
	return nil
}

// getKubeConfigSpecs returns all KubeConfigSpecs actualized to the context of the current InitConfiguration
// NB. this method holds the information about how kubeadm creates kubeconfig files.
func getKubeConfigSpecs(cfg *kubeadmapi.InitConfiguration) (map[string]*kubeConfigSpec, error) {
    //这边caCert，cakey是一个x509的结构体，TryLoadCertAndKeyFromDisk 函数 pathjoin 出文件后，
    //通过 read 读取文件进行了 ParseCertsPEM(pemBlock) 转换成结构体
	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(cfg.CertificatesDir, kubeadmconstants.CACertAndKeyBaseName)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create a kubeconfig; the CA files couldn't be loaded")
	}
    //这边CheckCertificatePeriodValidity函数里验证了结构体里 cert.NotBefore 信息
	// Validate period
	certsphase.CheckCertificatePeriodValidity(kubeadmconstants.CACertAndKeyBaseName, caCert)

    //getKubeConfigSpecsBase为/etc/kubernetes/*.config里面结构的一部分，这边就是获取一部分结构体
	configs, err := getKubeConfigSpecsBase(cfg)
	if err != nil {
		return nil, err
	}
	for _, spec := range configs {
        //把ca.crt，ca.key转换出来的后传给了spec.CACert，spec.ClientCertAuth.CAKey 
		spec.CACert = caCert
		spec.ClientCertAuth.CAKey = caKey
	}
    //返回的是一个有数据的结构体
	return configs, nil
}

//这边就是需要输入到 /etc/kubernetes/*.config 的文件的一些具体结构体内容
return map[string]*kubeConfigSpec{
		kubeadmconstants.AdminKubeConfigFileName: {
			APIServer:  controlPlaneEndpoint,
			ClientName: "kubernetes-admin",
			ClientCertAuth: &clientCertAuth{
				Organizations: []string{kubeadmconstants.SystemPrivilegedGroup},
			},
		},
		kubeadmconstants.KubeletKubeConfigFileName: {
			APIServer:  controlPlaneEndpoint,
			ClientName: fmt.Sprintf("%s%s", kubeadmconstants.NodesUserPrefix, cfg.NodeRegistration.Name),
			ClientCertAuth: &clientCertAuth{
				Organizations: []string{kubeadmconstants.NodesGroup},
			},
		},
		kubeadmconstants.ControllerManagerKubeConfigFileName: {
			APIServer:      localAPIEndpoint,
			ClientName:     kubeadmconstants.ControllerManagerUser,
			ClientCertAuth: &clientCertAuth{},
		},
		kubeadmconstants.SchedulerKubeConfigFileName: {
			APIServer:      localAPIEndpoint,
			ClientName:     kubeadmconstants.SchedulerUser,
			ClientCertAuth: &clientCertAuth{},
		},
	}, nil

//这边就是需要输入到 /etc/kubernetes/*.config 的文件的一些具体结构体内容
return map[string]*kubeConfigSpec{
    return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
```