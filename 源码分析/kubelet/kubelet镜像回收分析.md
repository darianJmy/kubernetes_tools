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

    // 这边初始化了两个结构体一个是 kubeletFlags，一个是 kubeletConfig
    // kubeletFlags 设置了一些默认值，这边主要分析 GC 代码，需要的参数为：
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
			// 解析参数，因为下面有 AddFlags 函数可以添加参数。
			// 这边解析的参数在一个数组里面。
			if err := cleanFlagSet.Parse(args); err != nil {
				...
			}
			...

            // 此处的 kubeletServer 结构体被实例化， kubeletFlags 结构体也被传给 kubeletServer。
			// construct a KubeletServer from kubeletFlags and kubeletConfig
			kubeletServer := &options.KubeletServer{
				KubeletFlags:         *kubeletFlags,
				KubeletConfiguration: *kubeletConfig,
			}

			// use kubeletServer to construct the default KubeletDeps
			//通过 kubeletServer 构造默认的 kubeletDeps。
			kubeletDeps, err := UnsecuredDependencies(kubeletServer, utilfeature.DefaultFeatureGate)
			...

            //执行了 Run 函数，完成一系列的初始化任务后，启动 kubelet 服务。
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

// run 函数里执行了 RunKubelet 函数。
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

    //执行死循环判断管道信号，保持服务一直运行。
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
    // NewMainKubelet 函数实例化了很多东西，包括 GC 要用到的参数，实例化了一些 Informer。
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

    //执行启动 GC 函数。
	k.StartGarbageCollection()

	return k, nil
}

// NewMainKubelet 函数主要作用就是实例化。
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

	//实例化了 containerGCPolicy 结构体。
	containerGCPolicy := kubecontainer.GCPolicy{
		MinAge:             minimumGCAge.Duration,
		MaxPerPodContainer: int(maxPerPodContainerCount),
		MaxContainers:      int(maxContainerCount),
	}
    //实例化了 imageGCPolicy。
	imageGCPolicy := images.ImageGCPolicy{
		MinAge:               kubeCfg.ImageMinimumGCAge.Duration,
		HighThresholdPercent: int(kubeCfg.ImageGCHighThresholdPercent),
		LowThresholdPercent:  int(kubeCfg.ImageGCLowThresholdPercent),
	}
    ...

    //实例化了 serviceLister Informer，
    //这里只是举例，还有其他 Informer。
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

        // sourcesReady 是一个结构体。
		sourcesReady:                            config.NewSourcesReady(kubeDeps.PodConfig.SeenAllSources),
		...
	}
    ...

	// setup containerGC
    // GC 代码部分用到，containerGC 是一个结构体。
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
// StartGarbageCollection 为 kubelet包里 Bootstrap 的方法，
// 这边启动了携程 go wait.Until(func(){}, ContainerGCPeriod, wait.NeverStop)，
// func(){} 是具体任务， ContainerGCPeriod 是每个多少时间循环一次， wait.NeverStop 是 channel，
// kl 结构体已经被实例化了，这边执行 kl.containerGC.GarbageCollect() 函数。
// StartGarbageCollection starts garbage collection threads.
func (kl *Kubelet) StartGarbageCollection() {
	loggedContainerGCFailure := false
	//启动携程
	go wait.Until(func() {
        //每个1分钟执行一次
		//处理containerGC
        //containerGC.GarbageCollect用法是，GarbageCollect 函数可读取 containerGC 结构体数据
		if err := kl.containerGC.GarbageCollect(); err != nil {
			//如果有 error，log 输出，event 输出
			klog.ErrorS(err, "Container garbage collection failed")
			kl.recorder.Eventf(kl.nodeRef, v1.EventTypeWarning, events.ContainerGCFailed, err.Error())
			loggedContainerGCFailure = true
		} else {
			var vLevel klog.Level = 4
			if loggedContainerGCFailure {
				vLevel = 1
				loggedContainerGCFailure = false
			}
			//如果没有报错,log等级为1。
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
		//处理imageGC。
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
```

### Kubelet 垃圾回收容器部分源码分析

```
// container_gc 包里 GC 接口的 GarbageCollect 方法，执行的是 runtime 包里的 GarbageCollect 方法。
func (cgc *realContainerGC) GarbageCollect() error {
	return cgc.runtime.GarbageCollect(cgc.policy, cgc.sourcesReadyProvider.AllReady(), false)
}

// runtime 包里的 GarbageCollect 方法， 执行的是 kuberuntime 包里 kubeGenericRuntimeManager 接口下的 GarbageCollect 方法。
func (m *kubeGenericRuntimeManager) GarbageCollect(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	return m.containerGC.GarbageCollect(gcPolicy, allSourcesReady, evictNonDeletedPods)
}

// GarbageCollect 函数里先定义了一个数组，
// 然后执行了 evictContainers 函数判断返回值，函数里执行了 removeContainers 函数，
// 然后执行 evictSandboxes 函数判断返回值，函数里执行了 removeSandboxes 函数，
// 然后执行 evictPodLogsDirectories 函数判断返回值, 函数里执行了 PodLogsDirectorie 函数。
func (cgc *containerGC) GarbageCollect(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	errors := []error{}
	// Remove evictable containers
    // 这边执行驱逐容器函数。
	if err := cgc.evictContainers(gcPolicy, allSourcesReady, evictNonDeletedPods); err != nil {
		errors = append(errors, err)
	}

	// Remove sandboxes with zero containers
	// 这边执行驱逐 sandboxes 容器函数。
	if err := cgc.evictSandboxes(evictNonDeletedPods); err != nil {
		errors = append(errors, err)
	}

	// Remove pod sandbox log directory
	// 执行删除 pod log 文件。
	if err := cgc.evictPodLogsDirectories(allSourcesReady); err != nil {
		errors = append(errors, err)
	}
	return utilerrors.NewAggregate(errors)
}
```

### Kubelet 垃圾回收容器部分之删除容器源码分析

```
// 驱逐容器函数。
// evict all containers that are evictable
// gcPolicy 是一个结构体，数据有： MinAge 默认是 0s， MaxPerPodContainer 默认是 1，	MaxContainers 默认是 -1。
// allSourcesReady 布尔值是获取的 cgc.sourcesReadyProvider.AllReady()， AllReady() 函数默认是 true，
// evictNonDeletedPods 布尔值直接就是 false。
func (cgc *containerGC) evictContainers(gcPolicy kubecontainer.GCPolicy, allSourcesReady bool, evictNonDeletedPods bool) error {
	// Separate containers by evict units.
    // gcPolicy.MinAge 默认 0s
    // 获取字典，状态非 1，创建时间小于现在时间
	evictUnits, err := cgc.evictableContainers(gcPolicy.MinAge)
	if err != nil {
		return err
	}
	
	// Remove deleted pod containers if all sources are ready.
    //判断是否执行 remove
	//默认是 true，如果这边是 true，下面两个移除被驱逐的容器方法实际上不产生任何效果，因为 delete 后字典为空
	if allSourcesReady {
		// evictUnits 是一个 map， key 唯一的，但是 unit是一个数组，可以有多个容器
		for key, unit := range evictUnits {

            // 判断容器 status 状态，如果是 IsEvicted 或 (IsDeleted 与 IsTerminated)，执行删除命令
			// evictNonDeletedPods 默认是 false，只判断状态是 IsEvicted 或 (IsDeleted 与 IsTerminated) 就行
			if cgc.podStateProvider.ShouldPodContentBeRemoved(key.uid) || (evictNonDeletedPods && cgc.podStateProvider.ShouldPodRuntimeBeRemoved(key.uid)) {
				
				// unit 是一个数组，len(unit) 计算 pod 下面有几个容器
				// 删除所有container
				cgc.removeOldestN(unit, len(unit)) // Remove all.
				//删除字典里 key，如果跟预期一样，evictUnits 字典应该为空
				delete(evictUnits, key)
			}
		}
	}

    // 判断 MaxPerPodContainer 是否大于等于 0，默认值是 1。
	// 所以此处的字典 evictUnits 应为空，所以 enforceMaxContainersPerEvictUnit 函数执行了个空
	// 驱逐每个 pod 里容器 > MaxPerPodContainer 的 Pod
	// 就是说如果 pod 里只有一个 容器不会被删除
	// Enforce max containers per evict unit.
	if gcPolicy.MaxPerPodContainer >= 0 {
		cgc.enforceMaxContainersPerEvictUnit(evictUnits, gcPolicy.MaxPerPodContainer)
	}

	// Enforce max total number of containers.
	// 判断 MaxContainers 是否大于等于 0 与 evictUnits 里 key 的数量大于 MaxContainers
	// MaxContainers 为 -1，就是没有限额，判断了 MaxContainers 是否大于等于 0 与 字典里 key 的数量是否大于 MaxContainers。 len(evictUnits[key])
	// 这里一般是不执行的，因为 if 语句有 false，所有不执行删除任务
	// 但是如果执行删除任务，逻辑跟 cgc.enforceMaxContainersPerEvictUnit(evictUnits, gcPolicy.MaxPerPodContainer) 一样
	if gcPolicy.MaxContainers >= 0 && evictUnits.NumContainers() > gcPolicy.MaxContainers {
		// Leave an equal number of containers per evict unit (min: 1).
		// 如果判断为 true，执行 if 里面内容
		// 如果 evictUnits 里 pod 的 container 的数量为 9，MaxContainers 为 1，就是说还要删除 8 个 container
		numContainersPerEvictUnit := gcPolicy.MaxContainers / evictUnits.NumEvictUnits()
		if numContainersPerEvictUnit < 1 {
			numContainersPerEvictUnit = 1
		}
		// 执行了 remove 函数，9 - 1 = 8 还要删除 8 个 containers
		cgc.enforceMaxContainersPerEvictUnit(evictUnits, numContainersPerEvictUnit)

		// If we still need to evict, evict oldest first.
		// 计算还有多少个容器
		numContainers := evictUnits.NumContainers()
		如果还大于 MaxContainers，就删除最旧的
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
// minAge 默认 0s。
// 函数处理了过滤掉不是 k8s 的 container， 并且把 containers 组合成 key 为 pod 带数组，数组里面为多个 container 的字典，返回。
func (cgc *containerGC) evictableContainers(minAge time.Duration) (containersByEvictUnit, error) {
    // getKubeletContainers 获取一个数组
	// 这边是获取所有 containers 
	containers, err := cgc.manager.getKubeletContainers(true)
	if err != nil {
		return containersByEvictUnit{}, err
	}
    // make 一个字典
	evictUnits := make(containersByEvictUnit)
    
	// newestGCTime 是当前时间 - 退出容器时间，
	// minAge 默认为 0s，
	// 所以不考虑容器退出后再等多久，再删除。
    // newestGCTime == time.Now()
	newestGCTime := time.Now().Add(-minAge)

    // 循环获取container
	for _, container := range containers {
		// Prune out running containers.
        // 判断container.State 判断状态如果是 1 ，结束当前循环，再次执行循环
		if container.State == runtimeapi.ContainerState_CONTAINER_RUNNING {
			continue
		}
        // 计算创建时间
		createdAt := time.Unix(0, container.CreatedAt)
        / 如果创建时间小于当前时间 - 退出容器时间，结束当前循环，再次执行循环
		if newestGCTime.Before(createdAt) {
			continue
		}
        // 获取标签信息
		// getContainerInfoFromLabels函数里判断了是否带有 k8s 的标签，
		// 如果带有 k8s 的标签，返回 value，如果不是 k8s 的标签返回 ""， 空
		labeledInfo := getContainerInfoFromLabels(container.Labels)
		containerInfo := containerGCInfo{
			id:         container.Id,
			name:       container.Metadata.Name,
			createTime: createdAt,
			//unknown 是布尔值，判断容器状态是不是 unknown
			unknown:    container.State == runtimeapi.ContainerState_CONTAINER_UNKNOWN,
		}
		key := evictUnit{
			uid:  labeledInfo.PodUID,
			name: containerInfo.name,
		}

        // append 到字典，这个时候 key如果是重复的，会追加 containerInfo 到 key 里面，
		// 如：map{ "pod1": ["container1", "contaner2"], "pod2": ["container3"]}
		// 不带标签的 container 也追加到字典里了，但是 key 为 ""，空，所以执行与丢弃效果一样
		evictUnits[key] = append(evictUnits[key], containerInfo)
	}
	// 返回字典
	return evictUnits, nil
}

// 获取容器列表，这边是获取所有容器列表
func (m *kubeGenericRuntimeManager) getKubeletContainers(allContainers bool) ([]*runtimeapi.Container, error) {
    // new 一个结构体，但结构体没数据
	filter := &runtimeapi.ContainerFilter{}
    // allContainers 是一个 bool 值，
	// 但是 cgc.manager.getKubeletContainers(true) 使用 true 调用的，
	// 默认不执行 if 语句里面内容，
	// 如果是 false 就是获取状态是 1 的容器。
	if !allContainers {
        //给结构体 State 值，runtimeapi.ContainerState_CONTAINER_RUNNING 默认为 1
		filter.State = &runtimeapi.ContainerStateValue{
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		}
	}
    //获取所有容器列表
	//这边就是获取所有 containers ，调用方式不是获取 informer ，而是直接调用 grpc， 获取容器运行时接口的 api。
	containers, err := m.runtimeService.ListContainers(filter)
	if err != nil {
		klog.ErrorS(err, "ListContainers failed")
		return nil, err
	}

	return containers, nil
}

// removeOldestN removes the oldest toRemove containers and returns the resulting slice.
// 执行删除函数
func (cgc *containerGC) removeOldestN(containers []containerGCInfo, toRemove int) []containerGCInfo {
	// Remove from oldest to newest (last to first).
	// 删除全部 container 函数执行此删除函数，toRemove 就是 len(containers)，此处又计算了一遍 len(containers) - toRemove 值为 0，所以删除全部 container 函数不保留任何容器无需排序
	// 删除保留部分container的函数执行此函数，
	// 所以 numToKeep 一定等于 0
	numToKeep := len(containers) - toRemove

	if numToKeep > 0 {
		sort.Sort(byCreated(containers))
	}
	// 从后往前读数据
	for i := len(containers) - 1; i >= numToKeep; i-- {
		// 如果 containers 状态是 unknown，执行 kill 函数再执行 remove 函数，防止直接 remove 删除不了
		if containers[i].unknown {
			// Containers in known state could be running, we should try
			// to stop it before removal.
			id := kubecontainer.ContainerID{
				Type: cgc.manager.runtimeName,
				ID:   containers[i].id,
			}
			message := "Container is in unknown state, try killing it before removal"
			if err := cgc.manager.killContainer(nil, id, containers[i].name, message, reasonUnknown, nil); err != nil {
				klog.ErrorS(err, "Failed to stop container", "containerID", containers[i].id)
				continue
			}
		}
		// 如果状态不是 unknown，执行 remove 函数
		if err := cgc.manager.removeContainer(containers[i].id); err != nil {
			klog.ErrorS(err, "Failed to remove container", "containerID", containers[i].id)
		}
	}

	// Assume we removed the containers so that we're not too aggressive.
	// 此处返回的一定是 containers[]，为空
	return containers[:numToKeep]
}

// 判断 pod 里最大可驱逐容器
// enforceMaxContainersPerEvictUnit enforces MaxPerPodContainer for each evictUnit.
func (cgc *containerGC) enforceMaxContainersPerEvictUnit(evictUnits containersByEvictUnit, MaxContainers int) {
	for key := range evictUnits {
        // 如果可驱逐的container 是 10， 那么就是 10 - 1， 可驱逐 9 个
		// 如果可驱逐的container 是 1， 那么就是 1 - 1， 可驱逐 0 个，containers 只有 1 个，不执行驱逐任务
		toRemove := len(evictUnits[key]) - MaxContainers

        // remove 容器，这边是倒叙删除，返回的是字典里没删除值
		if toRemove > 0 {
			evictUnits[key] = cgc.removeOldestN(evictUnits[key], toRemove)
		}
	}
}
```

### Kubelet 垃圾回收容器部分之删除 Sandbox 源码分析

```
// 容器删除了后，删除沙箱
// 驱逐Sandbox
// 如果删除容器函数生效，此处已经没有 Sandbox 可以删除了
func (cgc *containerGC) evictSandboxes(evictNonDeletedPods bool) error {
	// getKubeletContainers 获取所有 Containers 数组。
	containers, err := cgc.manager.getKubeletContainers(true)
	if err != nil {
		return err
	}
	//getKubeletSandboxes 获取所有 Sandboxes 数组
	sandboxes, err := cgc.manager.getKubeletSandboxes(true)
	if err != nil {
		return err
	}

	//定义一个 map
	// collect all the PodSandboxId of container
	sandboxIDs := sets.NewString()
	for _, container := range containers {
		//一直写入 containers 的 PodSandboxId
		sandboxIDs.Insert(container.PodSandboxId)
	}

	//定义一个 map
	sandboxesByPod := make(sandboxesByPodUID)
	//处理单个 sandbox
	for _, sandbox := range sandboxes {
		podUID := types.UID(sandbox.Metadata.Uid)
		sandboxInfo := sandboxGCInfo{
			id:         sandbox.Id,
			createTime: time.Unix(0, sandbox.CreatedAt),
		}

		// Set ready sandboxes to be active.
		// 如果 sandbox 的状态是 ready，sandboxInfo.active = true
		if sandbox.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			sandboxInfo.active = true
		}

		// Set sandboxes that still have containers to be active.
		// container数组里 sandbox.Id 与 sandbox 数组里 sandbox.Id 相同，设置为 true，存活状态
		if sandboxIDs.Has(sandbox.Id) {
			sandboxInfo.active = true
		}
		//给 sandboxesByPod 结构体加值，此处 podUID 为 key，value 为数组，一个 podUID 可以有多个 sandboxInfo
		sandboxesByPod[podUID] = append(sandboxesByPod[podUID], sandboxInfo)
	}

	for podUID, sandboxes := range sandboxesByPod {
		// 判断容器 status 状态，如果是 IsEvicted 或 (IsDeleted 与 IsTerminated) 与 evictNonDeletedPods 的布尔值
		// 如果为 true 执行 cgc.removeOldestNSandboxes(sandboxes, len(sandboxes)，删除所有 sandboxes
		// 如果为 false 执行 cgc.removeOldestNSandboxes(sandboxes, len(sandboxes)-1，-1 就是为了保留每个 pod 一个 sandbox 存在
		if cgc.podStateProvider.ShouldPodContentBeRemoved(podUID) || (evictNonDeletedPods && cgc.podStateProvider.ShouldPodRuntimeBeRemoved(podUID)) {
			// Remove all evictable sandboxes if the pod has been removed.
			// Note that the latest dead sandbox is also removed if there is
			// already an active one.
			cgc.removeOldestNSandboxes(sandboxes, len(sandboxes))
		} else {
			// Keep latest one if the pod still exists.
			//保留，因为 len(sandboxes)-1 为 0
			cgc.removeOldestNSandboxes(sandboxes, len(sandboxes)-1)
		}
	}
	return nil
}

// 获取 Sandboxes 数组的函数
func (m *kubeGenericRuntimeManager) getKubeletSandboxes(all bool) ([]*runtimeapi.PodSandbox, error) {
	var filter *runtimeapi.PodSandboxFilter
	//函数传参为 true
	if !all {
		readyState := runtimeapi.PodSandboxState_SANDBOX_READY
		filter = &runtimeapi.PodSandboxFilter{
			State: &runtimeapi.PodSandboxStateValue{
				State: readyState,
			},
		}
	}
	// 获取所有 sandbox 数组
	resp, err := m.runtimeService.ListPodSandbox(filter)
	if err != nil {
		klog.ErrorS(err, "Failed to list pod sandboxes")
		return nil, err
	}

	return resp, nil
}

// 删除 sandboxes 任务
func (cgc *containerGC) removeOldestNSandboxes(sandboxes []sandboxGCInfo, toRemove int) {
	//通过计算判断删除多少个 containers， 如果 2 - 2 就是一个不剩，如果 2-(2-1) 就是剩余 1 个
	numToKeep := len(sandboxes) - toRemove
	//如果剩余 1 个，就排序，判断哪个不删
	if numToKeep > 0 {
		sort.Sort(sandboxByCreated(sandboxes))
	}
	// Remove from oldest to newest (last to first).
	//倒叙删除
	for i := len(sandboxes) - 1; i >= numToKeep; i-- {
		if !sandboxes[i].active {
			cgc.removeSandbox(sandboxes[i].id)
		}
	}
}
```

### Kubelet 垃圾回收容器部分之删除容器源码分析

```
// 驱逐LogsDir
func (cgc *containerGC) evictPodLogsDirectories(allSourcesReady bool) error {
	// 获取 os 方法
	osInterface := cgc.manager.osInterface
	// 跟删除所有 containers 逻辑一样
	if allSourcesReady {
		// Only remove pod logs directories when all sources are ready.
		// 读取 /var/log/pods 目录
		dirs, err := osInterface.ReadDir(podLogsRootDirectory)
		if err != nil {
			return fmt.Errorf("failed to read podLogsRootDirectory %q: %v", podLogsRootDirectory, err)
		}
		// 处理单个 dir
		for _, dir := range dirs {
			// 获取名字
			name := dir.Name()
			// 获取 UID， parsePodUIDFromLogsDirectory 函数处理了切片，获取最后一个 _ 以后的值
			podUID := parsePodUIDFromLogsDirectory(name)
			// 判断 pod 状态，如果 run，重新开始执行循环
			if !cgc.podStateProvider.ShouldPodContentBeRemoved(podUID) {
				continue
			}
			klog.V(4).InfoS("Removing pod logs", "podUID", podUID)
			// 删除 log 文件
			err := osInterface.RemoveAll(filepath.Join(podLogsRootDirectory, name))
			if err != nil {
				klog.ErrorS(err, "Failed to remove pod logs directory", "path", name)
			}
		}
	}
	// 一些输出处理
	...
	return nil
}
```

### Kubelet 驱逐镜像部分源码分析

```
// 镜像部分GC
func (im *realImageGCManager) GarbageCollect() error {
	// Get disk usage on disk holding images.
	// 获取 image filesystem 文件系统的一些信息，也有可能是 nil
	// 所以下面判断 fsStats 是 nil 还是有数据的结构体
	fsStats, err := im.statsProvider.ImageFsStats()
	if err != nil {
		return err
	}
	// 定义两个变量
	var capacity, available int64
	// 如果文件系统总量不为 nil，传值
	if fsStats.CapacityBytes != nil {
		capacity = int64(*fsStats.CapacityBytes)
	}
	// 如果文件系统可用量不为 nil，传值
	if fsStats.AvailableBytes != nil {
		available = int64(*fsStats.AvailableBytes)
	}

	// 可用量 > 总量，总量传给可用量，矫正数据
	// 可用量不应该大于总量啊？
	if available > capacity {
		klog.InfoS("Availability is larger than capacity", "available", available, "capacity", capacity)
		// 
		available = capacity
	}

	// Check valid capacity.
	// 总量为 0，返回erro，记录事件
	if capacity == 0 {
		err := goerrors.New("invalid capacity 0 on image filesystem")
		im.recorder.Eventf(im.nodeRef, v1.EventTypeWarning, events.InvalidDiskCapacity, err.Error())
		return err
	}

	// If over the max threshold, free enough to place us at the lower threshold.
	// 计算使用的百分比，
	// 如果 100 - （可用量 1 * 100 / 总量 10），百分比为 90
	usagePercent := 100 - int(available*100/capacity)
	// 如果百分比大于等于设置的文件系统最多百分比 85，就执行 GC 具体删除任务
	if usagePercent >= im.policy.HighThresholdPercent {
		// 判断超出多少
		// 如果 10 * （100-80）/ 100 - 1，amountToFree 就是 1 ，就是超出 百分之 1
		amountToFree := capacity*int64(100-im.policy.LowThresholdPercent)/100 - available
		klog.InfoS("Disk usage on image filesystem is over the high threshold, trying to free bytes down to the low threshold", "usage", usagePercent, "highThreshold", im.policy.HighThresholdPercent, "amountToFree", amountToFree, "lowThreshold", im.policy.LowThresholdPercent)
		// 清理空间
		freed, err := im.freeSpace(amountToFree, time.Now())
		if err != nil {
			return err
		}
		// 如果清理过了还是小于超出，报错
		if freed < amountToFree {
			err := fmt.Errorf("failed to garbage collect required amount of images. Wanted to free %d bytes, but freed %d bytes", amountToFree, freed)
			im.recorder.Eventf(im.nodeRef, v1.EventTypeWarning, events.FreeDiskSpaceFailed, err.Error())
			return err
		}
	}

	return nil
}

// ImageFsStats returns the stats of the image filesystem.
// 获取所有 image
func (p *criStatsProvider) ImageFsStats() (*statsapi.FsStats, error) {
	// 这边调用 grpc，获取一个数组，值为 image filesystem 的统计信息
	// 该数组就一个下标，为 0， 值为 image filesystem 的结构体，
	resp, err := p.imageService.ImageFsInfo()
	if err != nil {
		return nil, err
	}

	// CRI may return the stats of multiple image filesystems but we only
	// return the first one.
	//
	// TODO(yguo0905): Support returning stats of multiple image filesystems.
	// 如果 resp == 0，意味着没有 images 文件系统，直接返回 nil
	if len(resp) == 0 {
		return nil, fmt.Errorf("imageFs information is unavailable")
	}
	// resp 数组里只有一个值，获取结构体
	fs := resp[0]
	s := &statsapi.FsStats{
		// Time 是时间戳，记录从现在开始 + 默认 2 分钟，
		// 就是现在传的镜像要过 2 分钟再处理。
		Time:      metav1.NewTime(time.Unix(0, fs.Timestamp)),
		// 已用的字节
		UsedBytes: &fs.UsedBytes.Value,
	}
	// 把 fs 使用索引节点信息传给 s
	if fs.InodesUsed != nil {
		s.InodesUsed = &fs.InodesUsed.Value
	}
	// 传了一个空结构体或者 nil
	imageFsInfo := p.getFsInfo(fs.GetFsId())
	// 如果 imageFsInfo 不等于空，赋值
	if imageFsInfo != nil {
		// The image filesystem id is unknown to the local node or there's
		// an error on retrieving the stats. In these cases, we omit those
		// stats and return the best-effort partial result. See
		// https://github.com/kubernetes/heapster/issues/1793.
		// 可用空间
		s.AvailableBytes = &imageFsInfo.Available
		// 总空间
		s.CapacityBytes = &imageFsInfo.Capacity
		s.InodesFree = imageFsInfo.InodesFree
		s.Inodes = imageFsInfo.Inodes
	}
	return s, nil
}

func (im *realImageGCManager) freeSpace(bytesToFree int64, freeTime time.Time) (int64, error) {
	// 返回了正在被使用的 images
	imagesInUse, err := im.detectImages(freeTime)
	if err != nil {
		return 0, err
	}

	im.imageRecordsLock.Lock()
	defer im.imageRecordsLock.Unlock()

	// Get all images in eviction order.
	// new 一个字典，长度为所有 image 的个数
	images := make([]evictionInfo, 0, len(im.imageRecords))
	for image, record := range im.imageRecords {
		// 如果所有 image 数组里有被使用的 image，
		// 重新执行 for 语句
		if isImageUsed(image, imagesInUse) {
			klog.V(5).InfoS("Image ID is being used", "imageID", image)
			continue
		}
		// Check if image is pinned, prevent garbage collection
		// 检查是否固定防止垃圾收集
		if record.pinned {
			klog.V(5).InfoS("Image is pinned, skipping garbage collection", "imageID", image)
			continue

		}
		// 定义 append，追加要删除的镜像
		images = append(images, evictionInfo{
			id:          image,
			imageRecord: *record,
		})
	}
	// 排序
	sort.Sort(byLastUsedAndDetected(images))

	// Delete unused images until we've freed up enough space.
	var deletionErrors []error
	spaceFreed := int64(0)
	for _, image := range images {
		klog.V(5).InfoS("Evaluating image ID for possible garbage collection", "imageID", image.id)
		// Images that are currently in used were given a newer lastUsed.
		// 如果 image 最新使用时间 等于 2 分钟，或者小于 2 分钟，不删除
		if image.lastUsed.Equal(freeTime) || image.lastUsed.After(freeTime) {
			klog.V(5).InfoS("Image ID was used too recently, not eligible for garbage collection", "imageID", image.id, "lastUsed", image.lastUsed, "freeTime", freeTime)
			continue
		}

		// Avoid garbage collect the image if the image is not old enough.
		// In such a case, the image may have just been pulled down, and will be used by a container right away.
		// 如果最新删除时间小于 0s，不删除，一般来说不应该有时间小于 0s
		if freeTime.Sub(image.firstDetected) < im.policy.MinAge {
			klog.V(5).InfoS("Image ID's age is less than the policy's minAge, not eligible for garbage collection", "imageID", image.id, "age", freeTime.Sub(image.firstDetected), "minAge", im.policy.MinAge)
			continue
		}

		// Remove image. Continue despite errors.
		klog.InfoS("Removing image to free bytes", "imageID", image.id, "size", image.size)
		// 移除 image
		err := im.runtime.RemoveImage(container.ImageSpec{Image: image.id})
		if err != nil {
			// 追加 error 到字典
			deletionErrors = append(deletionErrors, err)
			continue
		}
		// 删除 imageRecords 字典 已经被删除的 image id
		delete(im.imageRecords, image.id)
		// freespace ++
		spaceFreed += image.size
		// 如果 spaceFreed > 超过的百分比，结束删除镜像的循环任务
		if spaceFreed >= bytesToFree {
			break
		}
	}
	// 如果有错误，返回 spaceFreed，error
	if len(deletionErrors) > 0 {
		return spaceFreed, fmt.Errorf("wanted to free %d bytes, but freed %d bytes space with errors in image deletion: %v", bytesToFree, spaceFreed, errors.NewAggregate(deletionErrors))
	}
	// 返回 spaceFreed，nil
	return spaceFreed, nil
}

// 删除image
func (im *realImageGCManager) detectImages(detectTime time.Time) (sets.String, error) {
	imagesInUse := sets.NewString()

	// Always consider the container runtime pod sandbox image in use
	// 先处理 sandbox 镜像，如果有就把它放入正在使用的字典中，如果没有这串代码等于无效
	imageRef, err := im.runtime.GetImageRef(container.ImageSpec{Image: im.sandboxImage})
	// 没有错误，就是获取到 sandbox，并且 imageRef != ""，就写入 imagesInUse 结构体中
	if err == nil && imageRef != "" {
		imagesInUse.Insert(imageRef)
	}
	// 获取所有 images
	images, err := im.runtime.ListImages()
	if err != nil {
		return imagesInUse, err
	}
	// 获取 pods
	pods, err := im.runtime.GetPods(true)
	if err != nil {
		return imagesInUse, err
	}

	// Make a set of images in use by containers.
	for _, pod := range pods {
		for _, container := range pod.Containers {
			klog.V(5).InfoS("Container uses image", "pod", klog.KRef(pod.Namespace, pod.Name), "containerName", container.Name, "containerImage", container.Image, "imageID", container.ImageID)
			// 把 pod 正在使用的写入镜像 imagesInUse
			imagesInUse.Insert(container.ImageID)
		}
	}

	// Add new images and record those being used.
	// 记录现在事件
	now := time.Now()
	// new 一个字典
	currentImages := sets.NewString()
	// 锁
	im.imageRecordsLock.Lock()
	defer im.imageRecordsLock.Unlock()
	for _, image := range images {
		klog.V(5).InfoS("Adding image ID to currentImages", "imageID", image.ID)
		//把image.ID，写入字典，key 为 空
		currentImages.Insert(image.ID)

		// New image, set it as detected now.
		// 初始化 imageRecords 结构体，因为判断是 非 ok
		if _, ok := im.imageRecords[image.ID]; !ok {
			klog.V(5).InfoS("Image ID is new", "imageID", image.ID)
			im.imageRecords[image.ID] = &imageRecord{
				firstDetected: detectTime,
			}
		}

		// Set last used time to now if the image is being used.
		// 判断 image.ID 是不是正在使用中的
		if isImageUsed(image.ID, imagesInUse) {
			klog.V(5).InfoS("Setting Image ID lastUsed", "imageID", image.ID, "lastUsed", now)
			// 是的，更新一下时间
			im.imageRecords[image.ID].lastUsed = now
		}

		klog.V(5).InfoS("Image ID has size", "imageID", image.ID, "size", image.Size)
		// 写入 imageRecords 大小
		im.imageRecords[image.ID].size = image.Size

		klog.V(5).InfoS("Image ID is pinned", "imageID", image.ID, "pinned", image.Pinned)
		im.imageRecords[image.ID].pinned = image.Pinned
	}

	// Remove old images from our records.
	for image := range im.imageRecords {
		if !currentImages.Has(image) {
			klog.V(5).InfoS("Image ID is no longer present; removing from imageRecords", "imageID", image)
			delete(im.imageRecords, image)
		}
	}

	return imagesInUse, nil
}

// get 一遍 image 信息，看一下有没有被删除或者 
func (m *kubeGenericRuntimeManager) GetImageRef(image kubecontainer.ImageSpec) (string, error) {
	// 根据 image 结构体条件去 get，如果没有错往下执行
	resp, err := m.imageService.ImageStatus(toRuntimeAPIImageSpec(image), false)
	if err != nil {
		klog.ErrorS(err, "Failed to get image status", "image", image.Image)
		return "", err
	}
	// 如果 resp.Image 为 nil，返回 "", nil
	// 此处判断有没有找到这个 image
	if resp.Image == nil {
		return "", nil
	}
	// 返回的是一个 string， nil
	return resp.Image.Id, nil
}

// 获取 image 结构体
func toRuntimeAPIImageSpec(imageSpec kubecontainer.ImageSpec) *runtimeapi.ImageSpec {
	// 定义一个变量为 annotations 的字典
	var annotations = make(map[string]string)
	// 看这个 image 有没有注解，如果有注解，把注解 key 与 value 写入 annotations
	if imageSpec.Annotations != nil {
		for _, a := range imageSpec.Annotations {
			annotations[a.Name] = a.Value
		}
	}
	// 返回 image 结构体，此处注解应该是用来区分是不是 k8s 的镜像
	return &runtimeapi.ImageSpec{
		Image:       imageSpec.Image,
		Annotations: annotations,
	}
}

// 获取 pods
// 整个代码不理解
func (m *kubeGenericRuntimeManager) GetPods(all bool) ([]*kubecontainer.Pod, error) {
	pods := make(map[kubetypes.UID]*kubecontainer.Pod)
	// all 为 true，就是获取所有 sandbox 容器
	// 应为每个 pod 都会有一个 sandbox，这样的话可以直接通过 sandbox metadata，namespace 获取 pod 信息
	sandboxes, err := m.getKubeletSandboxes(all)
	if err != nil {
		return nil, err
	}
	for i := range sandboxes {
		s := sandboxes[i]
		if s.Metadata == nil {
			klog.V(4).InfoS("Sandbox does not have metadata", "sandbox", s)
			continue
		}
		// podUID 是 sandbox.Metadata.Uid
		podUID := kubetypes.UID(s.Metadata.Uid)
		// 判断有没有 pods 字典有没有 podUID，这里默认为没有，因为 pod 是 make 出来的，所以默认会写值
		// 如果程序有点问题出现多个 sandbox 在一个 pod 里，这里不处理只处理一个就够了，因为是要获取 pod 的信息
		if _, ok := pods[podUID]; !ok {
			pods[podUID] = &kubecontainer.Pod{
				ID:        podUID,
				Name:      s.Metadata.Name,
				Namespace: s.Metadata.Namespace,
			}
		}
		// p 是 value
		// 不理解整个 p 的作用，是 for 循环里面的，而且不是指针
		p := pods[podUID]
		// 返回一个结构体
		converted, err := m.sandboxToKubeContainer(s)
		if err != nil {
			klog.V(4).InfoS("Convert sandbox of pod failed", "runtimeName", m.runtimeName, "sandbox", s, "podUID", podUID, "err", err)
			continue
		}
		// 
		p.Sandboxes = append(p.Sandboxes, converted)
	}

	// 获取所有容器
	containers, err := m.getKubeletContainers(all)
	if err != nil {
		return nil, err
	}
	// 跟获取所有 sandbox 一样逻辑
	for i := range containers {
		c := containers[i]
		if c.Metadata == nil {
			klog.V(4).InfoS("Container does not have metadata", "container", c)
			continue
		}
		// 此处跟删除 container 部分一样，判断容器有没有 k8s 标签
		labelledInfo := getContainerInfoFromLabels(c.Labels)
		pod, found := pods[labelledInfo.PodUID]
		if !found {
			pod = &kubecontainer.Pod{
				ID:        labelledInfo.PodUID,
				Name:      labelledInfo.PodName,
				Namespace: labelledInfo.PodNamespace,
			}
			pods[labelledInfo.PodUID] = pod
		}

		converted, err := m.toKubeContainer(c)
		if err != nil {
			klog.V(4).InfoS("Convert container of pod failed", "runtimeName", m.runtimeName, "container", c, "podUID", labelledInfo.PodUID, "err", err)
			continue
		}

		pod.Containers = append(pod.Containers, converted)
	}

	// Convert map to list.
	var result []*kubecontainer.Pod
	for _, pod := range pods {
		result = append(result, pod)
	}

	return result, nil
}
```
