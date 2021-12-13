# NewCertsPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go 建议先看看cobra
##### NewKubeletStartPhase平面主要工作是创建kubelet启动的config文件与环境变量文件

##### 1、KubeletStart 平面不涉及子平面，runKubeletStart 函数负责 Print 提示与创建任务
```
return workflow.Phase{
		Name:    "kubelet-start",
		Short:   "Write kubelet settings and (re)start the kubelet",
		Long:    "Write a file with KubeletConfiguration and an environment file with node specific kubelet settings, and then (re)start kubelet.",
		Example: kubeletStartPhaseExample,
		Run:     runKubeletStart,
		InheritFlags: []string{
			options.CfgPath,
			options.NodeCRISocket,
			options.NodeName,
		},
	}
```

##### 2、runKubeletStart 函数会先检测 Print 一些数据作为的提示，输出后会检测初始化 DryRun 状态执行相应函数，并创建环境变量
```
	// First off, configure the kubelet. In this short timeframe, kubeadm is trying to stop/restart the kubelet
	// Try to stop the kubelet service so no race conditions occur when configuring it
	//data.DruRun 为 init 平面初始化的布尔值，如果执行 init 命令没有带该参数，默认 false
	//TryStopKubelet 为调用 system 函数执行 stop kubelet 的操作
	if !data.DryRun() {
		klog.V(1).Infoln("Stopping the kubelet")
		kubeletphase.TryStopKubelet()
	}

	//if err := initSystem.ServiceStop("kubelet"); err != nil {
	//	fmt.Printf("[kubelet-start] WARNING: unable to stop the kubelet service momentarily: [%v]\n", err)
	//}

	//这边是创建了两个环境变量，分别为 kubeadm-flags.env、config.yaml
	//kubeadm-flags.env 里面大致内容为判断运行时是 docker，网络插件等于cni、opt.pauseImage 不等于空pod-infra-container-image 等于 opt.pauseImage、register-with-taints、hostname-override
	if err := kubeletphase.WriteKubeletDynamicEnvFile(&data.Cfg().ClusterConfiguration, &data.Cfg().NodeRegistration, false, data.KubeletDir()); err != nil {
		return errors.Wrap(err, "error writing a dynamic environment file for the kubelet")
	}

	//这边初通过把 &data.Cfg().ClusterConfiguration 值写入 config.yaml 文件中
	// Write the kubelet configuration file to disk.
	if err := kubeletphase.WriteConfigToDisk(&data.Cfg().ClusterConfiguration, data.KubeletDir()); err != nil {
		return errors.Wrap(err, "error writing kubelet configuration to disk")
	}

	// Try to start the kubelet service in case it's inactive
	//启动 kubelet
	if !data.DryRun() {
		fmt.Println("[kubelet-start] Starting the kubelet")
		kubeletphase.TryStartKubelet()
	}


```