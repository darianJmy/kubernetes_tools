# Kubelet镜像回收分析

## 环境信息
- kubernetes: v1.23.1

## Kubelet通过读取配置文件里面的参数进行垃圾回收

```
kubelet 中与容器垃圾回收有关的主要有以下三个参数:
--maximum-dead-containers-per-container: 表示一个 pod 最多可以保存多少个已经停止的容器，默认为1；（maxPerPodContainerCount）
--maximum-dead-containers：一个 node 上最多可以保留多少个已经停止的容器，默认为 -1，表示没有限制；
--minimum-container-ttl-duration：已经退出的容器可以存活的最小时间，默认为 0s；
与镜像回收有关的主要有以下三个参数：

--image-gc-high-threshold：当 kubelet 磁盘达到多少时，kubelet 开始回收镜像，默认为 85% 开始回收，根目录以及数据盘；
--image-gc-low-threshold：回收镜像时当磁盘使用率减少至多少时停止回收，默认为 80%；
--minimum-image-ttl-duration：未使用的镜像在被回收前的最小存留时间，默认为 2m0s；
kubelet 中容器回收过程如下: pod 中的容器退出时间超过--minimum-container-ttl-duration后会被标记为可回收，一个 pod 中最多可以保留--maximum-dead-containers-per-container个已经停止的容器，一个 node 上最多可以保留--maximum-dead-containers个已停止的容器。在回收容器时，kubelet 会按照容器的退出时间排序，最先回收退出时间最久的容器。需要注意的是，kubelet 在回收时会将 pod 中的 container 与 sandboxes 分别进行回收，且在回收容器后会将其对应的 log dir 也进行回收；

kubelet 中镜像回收过程如下: 当容器镜像挂载点文件系统的磁盘使用率大于--image-gc-high-threshold时（containerRuntime 为 docker 时，镜像存放目录默认为 /var/lib/docker），kubelet 开始删除节点中未使用的容器镜像，直到磁盘使用率降低至--image-gc-low-threshold 时停止镜像的垃圾回收。
```

## Kubelet GarbageCollect 源码分析

### Kubelet Cobra 部分源码分析

```
func NewKubeletCommand() *cobra.Command {
	...
    //这边初始化了两个结构体一个是 kubeletFlags，一个是 kubeletConfig
    //设置了一些默认值，这边主要分析 GC 代码，需要的参数为：
    //  MaxContainerCount:       -1,
	//	MaxPerPodContainerCount: 1,
	//	MinimumGCAge:            metav1.Duration{Duration: 0},

	kubeletFlags := options.NewKubeletFlags()
	kubeletConfig, err := options.NewKubeletConfiguration()
	...

	cmd := &cobra.Command{
		Use: componentKubelet,
		...
		Run: func(cmd *cobra.Command, args []string) {
			// initial flag parse, since we disable cobra's flag parsing
			//解析参数。
			if err := cleanFlagSet.Parse(args); err != nil {
				...
			}
            ...
            //此处的 kubeletServer 结构体被实例化， kubeletFlags 结构体也被传给 kubeletServer。
			// construct a KubeletServer from kubeletFlags and kubeletConfig
			kubeletServer := &options.KubeletServer{
				KubeletFlags:         *kubeletFlags,
				KubeletConfiguration: *kubeletConfig,
			}
			// use kubeletServer to construct the default KubeletDeps
			//通过 kubeletServer 构造默认的 kubeletDeps。
			kubeletDeps, err := UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
			...
            //这边执行了 Run 函数，启动 kubelet 服务。
			// run the kubelet
			if err := Run(ctx, kubeletServer, kubeletDeps, utilfeature.DefaultFeatureGate); err != nil {
				klog.ErrorS(err, "Failed to run kubelet")
				os.Exit(1)
			}
		},
	}

	// keep cleanFlagSet separate, so Cobra doesn't pollute it with the global flags
	kubeletFlags.AddFlags(cleanFlagSet)
	options.AddKubeletConfigFlags(cleanFlagSet, kubeletConfig)
	options.AddGlobalFlags(cleanFlagSet)
	cleanFlagSet.BoolP("help", "h", false, fmt.Sprintf("help for %s", cmd.Name()))
    ...
	return cmd
}
```

### Kubelet Run 部分源码分析

```
// 这个 Run 函数里执行了 run 函数。
func Run(ctx context.Context, s *options.KubeletServer, kubeDeps *kubelet.Dependencies, featureGate featuregate.FeatureGate) error {
    ...
    if err := run(ctx, s, kubeDeps, featureGate); err != nil {
		return fmt.Errorf("failed to run Kubelet: %w", err)
	}
    ...
}

// run 函数里执行了 RunKubelet 函数
func run(ctx context.Context, s *options.KubeletServer, kubeDeps *kubelet.Dependencies, featureGate featuregate.FeatureGate) (err error) {
    //在 RunKubelet 函数之上，是一些完善 s 结构体与初始化的操作。
    ...
    if err := RunKubelet(s, kubeDeps, s.RunOnce); err != nil {
		return err
	}
    ...
}

// RunKubelet 函数里有 createAndInitKubelet 函数，把一些值传递给函数执行后返回结果给 k 变量后，k 就有了结构体的某个方法，后续可以 k.Run 启动 kubelet，
// 但是这里要看的是 createAndInitKubelet 函数。
func RunKubelet(kubeServer *options.KubeletServer, kubeDeps *kubelet.Dependencies, runOnce bool) error {
    //
    ...
    k, err := createAndInitKubelet(&kubeServer.KubeletConfiguration,
		kubeDeps,
		&kubeServer.ContainerRuntimeOptions,
		kubeServer.ContainerRuntime,
		hostname,
		hostnameOverridden,
		nodeName,
		nodeIPs,
		kubeServer.ProviderID,
		kubeServer.CloudProvider,
		kubeServer.CertDirectory,
		kubeServer.RootDirectory,
		kubeServer.ImageCredentialProviderConfigFile,
		kubeServer.ImageCredentialProviderBinDir,
		kubeServer.RegisterNode,
		kubeServer.RegisterWithTaints,
		kubeServer.AllowedUnsafeSysctls,
		kubeServer.ExperimentalMounterPath,
		kubeServer.KernelMemcgNotification,
		kubeServer.ExperimentalCheckNodeCapabilitiesBeforeMount,
		kubeServer.ExperimentalNodeAllocatableIgnoreEvictionThreshold,
		kubeServer.MinimumGCAge,
		kubeServer.MaxPerPodContainerCount,
		kubeServer.MaxContainerCount,
		kubeServer.MasterServiceNamespace,
		kubeServer.RegisterSchedulable,
		kubeServer.KeepTerminatedPodVolumes,
		kubeServer.NodeLabels,
		kubeServer.NodeStatusMaxImages,
		kubeServer.KubeletFlags.SeccompDefault || kubeServer.KubeletConfiguration.SeccompDefault,
	)
	if err != nil {
		return fmt.Errorf("failed to create kubelet: %w", err)
	}
    //执行启动服务，执行活检。
    ...
    //最后执行死循环判断管道信号，保持服务一直运行。
}

// createAndInitKubelet 接受传参后，执行 kubelet 包的 NewMainKubelet 函数，初始化结构体。
// k 获得结构体数据后，有了执行 StartGarbageCollection 的方法。
func createAndInitKubelet(kubeCfg *kubeletconfiginternal.KubeletConfiguration,
	kubeDeps *kubelet.Dependencies,
	crOptions *config.ContainerRuntimeOptions,
	containerRuntime string,
	hostname string,
	hostnameOverridden bool,
	nodeName types.NodeName,
	nodeIPs []net.IP,
	providerID string,
	cloudProvider string,
	certDirectory string,
	rootDirectory string,
	imageCredentialProviderConfigFile string,
	imageCredentialProviderBinDir string,
	registerNode bool,
	registerWithTaints []v1.Taint,
	allowedUnsafeSysctls []string,
	experimentalMounterPath string,
	kernelMemcgNotification bool,
	experimentalCheckNodeCapabilitiesBeforeMount bool,
	experimentalNodeAllocatableIgnoreEvictionThreshold bool,
	minimumGCAge metav1.Duration,
	maxPerPodContainerCount int32,
	maxContainerCount int32,
	masterServiceNamespace string,
	registerSchedulable bool,
	keepTerminatedPodVolumes bool,
	nodeLabels map[string]string,
	nodeStatusMaxImages int32,
	seccompDefault bool,
) (k kubelet.Bootstrap, err error) {
    // NewMainKubelet 函数实例化了很多东西，包括 GC 要用到的参数，实例化了 SVC、POD等 Informer，
    k, err = kubelet.NewMainKubelet(kubeCfg,
		kubeDeps,
		crOptions,
		containerRuntime,
		hostname,
		hostnameOverridden,
		nodeName,
		nodeIPs,
		providerID,
		cloudProvider,
		certDirectory,
		rootDirectory,
		imageCredentialProviderConfigFile,
		imageCredentialProviderBinDir,
		registerNode,
		registerWithTaints,
		allowedUnsafeSysctls,
		experimentalMounterPath,
		kernelMemcgNotification,
		experimentalCheckNodeCapabilitiesBeforeMount,
		experimentalNodeAllocatableIgnoreEvictionThreshold,
		minimumGCAge,
		maxPerPodContainerCount,
		maxContainerCount,
		masterServiceNamespace,
		registerSchedulable,
		keepTerminatedPodVolumes,
		nodeLabels,
		nodeStatusMaxImages,
		seccompDefault,
	)
	if err != nil {
		return nil, err
	}

	k.BirthCry()
    //执行启动 GC 函数
	k.StartGarbageCollection()

	return k, nil
}

// NewMainKubelet 函数主要作用就是实例化
func NewMainKubelet(kubeCfg *kubeletconfiginternal.KubeletConfiguration,
	kubeDeps *Dependencies,
	crOptions *config.ContainerRuntimeOptions,
	containerRuntime string,
	hostname string,
	hostnameOverridden bool,
	nodeName types.NodeName,
	nodeIPs []net.IP,
	providerID string,
	cloudProvider string,
	certDirectory string,
	rootDirectory string,
	imageCredentialProviderConfigFile string,
	imageCredentialProviderBinDir string,
	registerNode bool,
	registerWithTaints []v1.Taint,
	allowedUnsafeSysctls []string,
	experimentalMounterPath string,
	kernelMemcgNotification bool,
	experimentalCheckNodeCapabilitiesBeforeMount bool,
	experimentalNodeAllocatableIgnoreEvictionThreshold bool,
	minimumGCAge metav1.Duration,
	maxPerPodContainerCount int32,
	maxContainerCount int32,
	masterServiceNamespace string,
	registerSchedulable bool,
	keepTerminatedPodVolumes bool,
	nodeLabels map[string]string,
	nodeStatusMaxImages int32,
	seccompDefault bool,
) (*Kubelet, error) {
	...
	//实例化了 containerGCPolicy
	containerGCPolicy := kubecontainer.GCPolicy{
		MinAge:             minimumGCAge.Duration,
		MaxPerPodContainer: int(maxPerPodContainerCount),
		MaxContainers:      int(maxContainerCount),
	}
    //实例化了 imageGCPolicy
	imageGCPolicy := images.ImageGCPolicy{
		MinAge:               kubeCfg.ImageMinimumGCAge.Duration,
		HighThresholdPercent: int(kubeCfg.ImageGCHighThresholdPercent),
		LowThresholdPercent:  int(kubeCfg.ImageGCLowThresholdPercent),
	}
    ...
    //实例化了 serviceLister Informer
    //以便后续 GC 函数使用 Informer 获取数据
    //这里是举例，还有containerLister等等 Informer
	var serviceLister corelisters.ServiceLister
	var serviceHasSynced cache.InformerSynced
	if kubeDeps.KubeClient != nil {
		kubeInformers := informers.NewSharedInformerFactory(kubeDeps.KubeClient, 0)
		serviceLister = kubeInformers.Core().V1().Services().Lister()
		serviceHasSynced = kubeInformers.Core().V1().Services().Informer().HasSynced
		kubeInformers.Start(wait.NeverStop)
	} else {
		serviceIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		serviceLister = corelisters.NewServiceLister(serviceIndexer)
		serviceHasSynced = func() bool { return true }
	}
    ...
	klet := &Kubelet{
		...
        // sourcesReady 是一个接口
		sourcesReady:                            config.NewSourcesReady(kubeDeps.PodConfig.SeenAllSources),
		...
	}
    ...
	// setup containerGC
    // GC 代码部分用到
    // containerGC 是一个结构体
	containerGC, err := kubecontainer.NewContainerGC(klet.containerRuntime, containerGCPolicy, klet.sourcesReady)
	if err != nil {
		return nil, err
	}
	klet.containerGC = containerGC
	
	imageManager, err := images.NewImageGCManager(klet.containerRuntime, klet.StatsProvider, kubeDeps.Recorder, nodeRef, imageGCPolicy, crOptions.PodSandboxImage)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize image manager: %v", err)
	}
	klet.imageManager = imageManager
    ... 

	return klet, nil
}
```

### Kubelet GC 部分源码分析

```
// StartGarbageCollection 为 kubelet包里 Bootstrap 的方法
// 这边启动了携程 go wait.Until(func(){}, ContainerGCPeriod, wait.NeverStop)
// func(){} 是具体任务， ContainerGCPeriod 是每个多少时间循环一次， wait.NeverStop 是 channel
// kl 结构体已经被实例化了，这边执行 kl.containerGC.GarbageCollect() 函数
// StartGarbageCollection starts garbage collection threads.
func (kl *Kubelet) StartGarbageCollection() {
	loggedContainerGCFailure := false
	//启动携程
	go wait.Until(func() {
        //每个1分钟执行一次
		//处理containerGC
        //containerGC.GarbageCollect，相当于 GarbageCollect 可读取 containerGC 结构体数据
		if err := kl.containerGC.GarbageCollect(); err != nil {
			klog.ErrorS(err, "Container garbage collection failed")
			kl.recorder.Eventf(kl.nodeRef, v1.EventTypeWarning, events.ContainerGCFailed, err.Error())
			loggedContainerGCFailure = true
		} else {
			var vLevel klog.Level = 4
			if loggedContainerGCFailure {
				vLevel = 1
				loggedContainerGCFailure = false
			}
			//如果没有报错,log等级为1
			klog.V(vLevel).InfoS("Container garbage collection succeeded")
		}
	}, ContainerGCPeriod, wait.NeverStop)

	// when the high threshold is set to 100, stub the image GC manager
	if kl.kubeletConfiguration.ImageGCHighThresholdPercent == 100 {
		klog.V(2).InfoS("ImageGCHighThresholdPercent is set 100, Disable image GC")
		return
	}

	prevImageGCFailed := false
	go wait.Until(func() {
		//处理imageGC
		if err := kl.imageManager.GarbageCollect(); err != nil {
			if prevImageGCFailed {
				klog.ErrorS(err, "Image garbage collection failed multiple times in a row")
				// Only create an event for repeated failures
				kl.recorder.Eventf(kl.nodeRef, v1.EventTypeWarning, events.ImageGCFailed, err.Error())
			} else {
				klog.ErrorS(err, "Image garbage collection failed once. Stats initialization may not have completed yet")
			}
			prevImageGCFailed = true
		} else {
			var vLevel klog.Level = 4
			if prevImageGCFailed {
				vLevel = 1
				prevImageGCFailed = false
			}

			klog.V(vLevel).InfoS("Image garbage collection succeeded")
		}
	}, ImageGCPeriod, wait.NeverStop)
}

// container_gc 包里 GC 接口的 GarbageCollect 方法，执行的是 runtime 包里的 GarbageCollect 方法
func (cgc *realContainerGC) GarbageCollect() error {
	return cgc.runtime.GarbageCollect(cgc.policy, cgc.sourcesReadyProvider.AllReady(), false)
}

// runtime 包里的 GarbageCollect 方法， 执行的是 kuberuntime 包里 kubeGenericRuntimeManager 接口下的 GarbageCollect 方法
func (m *kubeGenericRuntimeManager) GarbageCollect(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	return m.containerGC.GarbageCollect(gcPolicy, allSourcesReady, evictNonDeletedPods)
}

// GarbageCollect 函数里先定义了一个数组，
// 然后执行了 evictContainers 函数判断返回值，函数里执行了 removeContainers 函数
// 然后执行 evictSandboxes 函数判断返回值，函数里执行了 removeSandboxes 函数
// 然后执行 evictPodLogsDirectories 函数判断返回值, 函数里执行了 PodLogsDirectorie 函数
func (cgc *containerGC) GarbageCollect(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	errors := []error{}
	// Remove evictable containers
    // 这边执行驱逐容器函数
	if err := cgc.evictContainers(gcPolicy, allSourcesReady, evictNonDeletedPods); err != nil {
		errors = append(errors, err)
	}

	// Remove sandboxes with zero containers
	if err := cgc.evictSandboxes(evictNonDeletedPods); err != nil {
		errors = append(errors, err)
	}

	// Remove pod sandbox log directory
	if err := cgc.evictPodLogsDirectories(allSourcesReady); err != nil {
		errors = append(errors, err)
	}
	return utilerrors.NewAggregate(errors)
}
```

### Kubelet 驱逐容器部分源码分析

```
// 驱逐容器函数
// evict all containers that are evictable
func (cgc *containerGC) evictContainers(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	// Separate containers by evict units.
    // gcPolicy.MinAge 默认 0s
    // 获取字典，状态非 run，创建时间小于现在时间
	evictUnits, err := cgc.evictableContainers(gcPolicy.MinAge)
	if err != nil {
		return err
	}

	// Remove deleted pod containers if all sources are ready.
    //判断是否执行 remove
	if allSourcesReady {
		for key, unit := range evictUnits {
            //执行 remove
			if cgc.podStateProvider.ShouldPodContentBeRemoved(key.uid) || (evictNonDeletedPods && cgc.podStateProvider.ShouldPodRuntimeBeRemoved(key.uid)) {
				cgc.removeOldestN(unit, len(unit)) // Remove all.
				delete(evictUnits, key)
			}
		}
	}

    // 判断是需要驱逐
	// Enforce max containers per evict unit.
	if gcPolicy.MaxPerPodContainer >= 0 {
		cgc.enforceMaxContainersPerEvictUnit(evictUnits, gcPolicy.MaxPerPodContainer)
	}

	// Enforce max total number of containers.
	if gcPolicy.MaxContainers >= 0 && evictUnits.NumContainers() > gcPolicy.MaxContainers {
		// Leave an equal number of containers per evict unit (min: 1).
		numContainersPerEvictUnit := gcPolicy.MaxContainers / evictUnits.NumEvictUnits()
		if numContainersPerEvictUnit < 1 {
			numContainersPerEvictUnit = 1
		}
		cgc.enforceMaxContainersPerEvictUnit(evictUnits, numContainersPerEvictUnit)

		// If we still need to evict, evict oldest first.
		numContainers := evictUnits.NumContainers()
		if numContainers > gcPolicy.MaxContainers {
			flattened := make([]containerGCInfo, 0, numContainers)
			for key := range evictUnits {
				flattened = append(flattened, evictUnits[key]...)
			}
			sort.Sort(byCreated(flattened))

			cgc.removeOldestN(flattened, numContainers-gcPolicy.MaxContainers)
		}
	}
	return nil
}

// 获取驱逐容器列表
func (cgc *containerGC) evictableContainers(minAge time.Duration) (containersByEvictUnit, error) {
    //getKubeletContainers 获取一个数组，
	containers, err := cgc.manager.getKubeletContainers(true)
	if err != nil {
		return containersByEvictUnit{}, err
	}
    // make 一个字典
	evictUnits := make(containersByEvictUnit)
    //minAge 默认为 0s，
    //newestGCTime == time.Now()
	newestGCTime := time.Now().Add(-minAge)

    //循环获取container
	for _, container := range containers {
		// Prune out running containers.
        // 判断container.State 判断状态如果是 1 ，结束当前循环，再次执行循环
		if container.State == runtimeapi.ContainerState_CONTAINER_RUNNING {
			continue
		}
        //计算创建时间
		createdAt := time.Unix(0, container.CreatedAt)
        /如果创建时间小于newestGCTime，结束当前循环，再次执行循环
		if newestGCTime.Before(createdAt) {
			continue
		}
        //获取标签的相关信息
		labeledInfo := getContainerInfoFromLabels(container.Labels)
		containerInfo := containerGCInfo{
			id:         container.Id,
			name:       container.Metadata.Name,
			createTime: createdAt,
			unknown:    container.State == runtimeapi.ContainerState_CONTAINER_UNKNOWN,
		}
		key := evictUnit{
			uid:  labeledInfo.PodUID,
			name: containerInfo.name,
		}
        //写到字典
		evictUnits[key] = append(evictUnits[key], containerInfo)
	}

	return evictUnits, nil
}

// 获取容器列表，这边是获取所有容器列表
func (m *kubeGenericRuntimeManager) getKubeletContainers(allContainers bool) ([]*runtimeapi.Container, error) {
    // new 一个结构体，但结构体没数据
	filter := &runtimeapi.ContainerFilter{}
    // allContainers 是一个 bool 值，但是 cgc.manager.getKubeletContainers(true) 使用 true 调用的，默认不执行 if 语句里面内容
	if !allContainers {
        //给结构体 State 值，runtimeapi.ContainerState_CONTAINER_RUNNING 默认为 1
		filter.State = &runtimeapi.ContainerStateValue{
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		}
	}
    //获取容器列表
	containers, err := m.runtimeService.ListContainers(filter)
	if err != nil {
		klog.ErrorS(err, "ListContainers failed")
		return nil, err
	}

	return containers, nil
}

//判断最大可驱逐容器
// enforceMaxContainersPerEvictUnit enforces MaxPerPodContainer for each evictUnit.
func (cgc *containerGC) enforceMaxContainersPerEvictUnit(evictUnits containersByEvictUnit, MaxContainers int) {
	for key := range evictUnits {
        //如果可驱逐的是 10， 那么就是 10 - 1， 可驱逐 9 个
		toRemove := len(evictUnits[key]) - MaxContainers

        // remove 容器，这边是按时间排序的
		if toRemove > 0 {
			evictUnits[key] = cgc.removeOldestN(evictUnits[key], toRemove)
		}
	}
}










//驱逐Sandbox
func (cgc *containerGC) evictSandboxes(evictNonDeletedPods bool) error {
	containers, err := cgc.manager.getKubeletContainers(true)
	if err != nil {
		return err
	}

	sandboxes, err := cgc.manager.getKubeletSandboxes(true)
	if err != nil {
		return err
	}

	// collect all the PodSandboxId of container
	sandboxIDs := sets.NewString()
	for _, container := range containers {
		sandboxIDs.Insert(container.PodSandboxId)
	}

	sandboxesByPod := make(sandboxesByPodUID)
	for _, sandbox := range sandboxes {
		podUID := types.UID(sandbox.Metadata.Uid)
		sandboxInfo := sandboxGCInfo{
			id:         sandbox.Id,
			createTime: time.Unix(0, sandbox.CreatedAt),
		}

		// Set ready sandboxes to be active.
		if sandbox.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			sandboxInfo.active = true
		}

		// Set sandboxes that still have containers to be active.
		if sandboxIDs.Has(sandbox.Id) {
			sandboxInfo.active = true
		}

		sandboxesByPod[podUID] = append(sandboxesByPod[podUID], sandboxInfo)
	}

	for podUID, sandboxes := range sandboxesByPod {
		if cgc.podStateProvider.ShouldPodContentBeRemoved(podUID) || (evictNonDeletedPods && cgc.podStateProvider.ShouldPodRuntimeBeRemoved(podUID)) {
			// Remove all evictable sandboxes if the pod has been removed.
			// Note that the latest dead sandbox is also removed if there is
			// already an active one.
			cgc.removeOldestNSandboxes(sandboxes, len(sandboxes))
		} else {
			// Keep latest one if the pod still exists.
			cgc.removeOldestNSandboxes(sandboxes, len(sandboxes)-1)
		}
	}
	return nil
}

// 驱逐LogsDir
func (cgc *containerGC) evictPodLogsDirectories(allSourcesReady bool) error {
	osInterface := cgc.manager.osInterface
	if allSourcesReady {
		// Only remove pod logs directories when all sources are ready.
		dirs, err := osInterface.ReadDir(podLogsRootDirectory)
		if err != nil {
			return fmt.Errorf("failed to read podLogsRootDirectory %q: %v", podLogsRootDirectory, err)
		}
		for _, dir := range dirs {
			name := dir.Name()
			podUID := parsePodUIDFromLogsDirectory(name)
			if !cgc.podStateProvider.ShouldPodContentBeRemoved(podUID) {
				continue
			}
			klog.V(4).InfoS("Removing pod logs", "podUID", podUID)
			err := osInterface.RemoveAll(filepath.Join(podLogsRootDirectory, name))
			if err != nil {
				klog.ErrorS(err, "Failed to remove pod logs directory", "path", name)
			}
		}
	}

	// Remove dead container log symlinks.
	// TODO(random-liu): Remove this after cluster logging supports CRI container log path.
	logSymlinks, _ := osInterface.Glob(filepath.Join(legacyContainerLogsDir, fmt.Sprintf("*.%s", legacyLogSuffix)))
	for _, logSymlink := range logSymlinks {
		if _, err := osInterface.Stat(logSymlink); os.IsNotExist(err) {
			if containerID, err := getContainerIDFromLegacyLogSymlink(logSymlink); err == nil {
				status, err := cgc.manager.runtimeService.ContainerStatus(containerID)
				if err != nil {
					// TODO: we should handle container not found (i.e. container was deleted) case differently
					// once https://github.com/kubernetes/kubernetes/issues/63336 is resolved
					klog.InfoS("Error getting ContainerStatus for containerID", "containerID", containerID, "err", err)
				} else if status.State != runtimeapi.ContainerState_CONTAINER_EXITED {
					// Here is how container log rotation works (see containerLogManager#rotateLatestLog):
					//
					// 1. rename current log to rotated log file whose filename contains current timestamp (fmt.Sprintf("%s.%s", log, timestamp))
					// 2. reopen the container log
					// 3. if #2 fails, rename rotated log file back to container log
					//
					// There is small but indeterministic amount of time during which log file doesn't exist (between steps #1 and #2, between #1 and #3).
					// Hence the symlink may be deemed unhealthy during that period.
					// See https://github.com/kubernetes/kubernetes/issues/52172
					//
					// We only remove unhealthy symlink for dead containers
					klog.V(5).InfoS("Container is still running, not removing symlink", "containerID", containerID, "path", logSymlink)
					continue
				}
			} else {
				klog.V(4).InfoS("Unable to obtain container ID", "err", err)
			}
			err := osInterface.Remove(logSymlink)
			if err != nil {
				klog.ErrorS(err, "Failed to remove container log dead symlink", "path", logSymlink)
			} else {
				klog.V(4).InfoS("Removed symlink", "path", logSymlink)
			}
		}
	}
	return nil
}
```

