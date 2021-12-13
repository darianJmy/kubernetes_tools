# NewWaitControlPlanePhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewPreflighPhase平面主要工作发送请求，等待一定时间内响应 200 

##### 1、NewWaitControlPlane 平面没有涉及子平面，运行内容全在 runWaitControlPlanePhase 函数中
```
	phase := workflow.Phase{
		Name:   "wait-control-plane",
		Run:    runWaitControlPlanePhase,
		Hidden: true,
	}
	return phase
```

##### 2、runWaitControlPlanePhase 函数等待服务启动
```
func runWaitControlPlanePhase(c workflow.RunData) error {
	data, ok := c.(InitData)
	if !ok {
		return errors.New("wait-control-plane phase invoked with an invalid data struct")
	}

	// If we're dry-running, print the generated manifests.
	// TODO: think of a better place to move this call - e.g. a hidden phase.
	if data.DryRun() {
		if err := dryrunutil.PrintFilesIfDryRunning(true /* needPrintManifest */, data.ManifestDir(), data.OutputWriter()); err != nil {
			return errors.Wrap(err, "error printing files on dryrun")
		}
	}

	// waiter holds the apiclient.Waiter implementation of choice, responsible for querying the API server in various ways and waiting for conditions to be fulfilled
	klog.V(1).Infoln("[wait-control-plane] Waiting for the API server to be healthy")

    //new 一个 client
	client, err := data.Client()
	if err != nil {
		return errors.Wrap(err, "cannot obtain client")
	}

	timeout := data.Cfg().ClusterConfiguration.APIServer.TimeoutForControlPlane.Duration
    //waiter 为一个结构体
	waiter, err := newControlPlaneWaiter(data.DryRun(), timeout, client, data.OutputWriter())
	if err != nil {
		return errors.Wrap(err, "error creating waiter")
	}

	fmt.Printf("[wait-control-plane] Waiting for the kubelet to boot up the control plane as static Pods from directory %q. This can take up to %v\n", data.ManifestDir(), timeout)
    //用来执行具体检测
    //WaitForKubeletAndFunc 函数具体检测了 kubelet服务 10248 端口，4 分钟内的状态
    //通过 client 获取 apiserver 的 /healthz 判断返回状态是不是 200
	if err := waiter.WaitForKubeletAndFunc(waiter.WaitForAPI); err != nil {
		context := struct {
			Error    string
			Socket   string
			IsDocker bool
		}{
			Error:    fmt.Sprintf("%v", err),
			Socket:   data.Cfg().NodeRegistration.CRISocket,
			IsDocker: data.Cfg().NodeRegistration.CRISocket == kubeadmconstants.DefaultDockerCRISocket,
		}

		kubeletFailTempl.Execute(data.OutputWriter(), context)
		return errors.New("couldn't initialize a Kubernetes cluster")
	}

	return nil
}

//// WaitForKubeletAndFunc waits primarily for the function f to execute, even though it might take some time. If that takes a long time, and the kubelet
// /healthz continuously are unhealthy, kubeadm will error out after a period of exponential backoff
func (w *KubeWaiter) WaitForKubeletAndFunc(f func() error) error {
    //这边用了管道，如果 kubelet 跟 apisever 有一个返回 error 都不行
	errorChan := make(chan error, 1)

	go func(errC chan error, waiter Waiter) {
		if err := waiter.WaitForHealthyKubelet(40*time.Second, fmt.Sprintf("http://localhost:%d/healthz", kubeadmconstants.KubeletHealthzPort)); err != nil {
			errC <- err
		}
	}(errorChan, w)

	go func(errC chan error, waiter Waiter) {
		// This main goroutine sends whatever the f function returns (error or not) to the channel
		// This in order to continue on success (nil error), or just fail if the function returns an error
		errC <- f()
	}(errorChan, w)

	// This call is blocking until one of the goroutines sends to errorChan
    //最终返回的是管道里的值
	return <-errorChan
}
```