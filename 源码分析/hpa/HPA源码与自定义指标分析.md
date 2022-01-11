# HPA源码与自定义指标分析

## 环境信息
- kubernetes: v1.23.1

## HPA实现过程是通过配置文件里面的指标去获取对象对应的值然后执行扩缩容
```
Horizo​​ntal Pod Autoscaler 根据资源使用情况自动扩展部署或副本集中 Pod 数量。
Horizo​​ntal Pod Autoscaler 功能最初是在 Kubernetes v1.1 中引入的，从那时起已经发展了很多。
HPA 的第 1 版根据观察到的 CPU 利用率和后来的内存使用情况扩展了 pod。
在 Kubernetes 1.6 中引入了一个新的 API 自定义指标 API，它允许 HPA 访问任意指标。Kubernetes 1.7 引入了聚合层，允许 3rd 方应用程序通过将自己注册为 API 附加组件来扩展 Kubernetes API。
自定义指标 API 和聚合层使 Prometheus 等监控系统能够将特定于应用程序的指标公开给 HPA 控制器。
Horizo​​ntal Pod Autoscaler 被实现为一个控制循环，它定期查询 Resource Metrics API 以获取 CPU/内存等核心指标，并通过 Custom Metrics API 查询特定于应用程序的指标。
```
#### 根据社区的案例进行HPA扩缩容测试
##### 克隆社区项目
```
$ git clone https://github.com/stefanprodan/k8s-prom-hpa.git
```
##### 进入项目安装metrics-server服务
```
$ kubectl create -f ./metrics-server
```
##### 这个metrics-server的api是否有指标
```
$ kubectl get --raw "/apis/metrics.k8s.io/v1beta1/" | jq .
```
##### 创建一个 deployment 服务
```
$ kubectl create -f ./podinfo/podinfo-svc.yaml,./podinfo/podinfo-dep.yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: podinfo
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: podinfo
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
        target:
          type: Utilization
          averageUtilization: 80
  - type: Resource
    resource:
      name: memory
        target:
          type: AverageValue
          averageValue: 200Mi
```
##### 创建一个 hpa 服务
```
$ kubectl create -f ./podinfo/podinfo-hpa.yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: podinfo
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: podinfo
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 80
    - type: Resource
      resource:
        name: memory
        target:
          type: AverageValue
          averageValue: 200Mi
```
##### 查询 hpa 服务
```
$ kubectl get hpa
NAME      REFERENCE            TARGETS                      MINPODS   MAXPODS   REPLICAS   AGE
podinfo   Deployment/podinfo   2826240 / 200Mi, 15% / 80%   2         10        2          5m
//查看 hpa 事件，可以看到 hpa 是根据 cpu、memory 的指标进行扩缩容的
$ kubectl describe hpa
Events:
  Type    Reason             Age   From                       Message
  ----    ------             ----  ----                       -------
  Normal  SuccessfulRescale  7m    horizontal-pod-autoscaler  New size: 4; reason: cpu resource utilization (percentage of request) above target
  Normal  SuccessfulRescale  3m    horizontal-pod-autoscaler  New size: 8; reason: cpu resource utilization (percentage of request) above target
```
##### 创建命名空间
```
$ kubectl create -f ./namespaces.yaml
```
##### 部署 prometheus 服务
```
$ kubectl create -f ./prometheus
```
#### 生成 prometheus 适配器所需 TLS
```
$ touch metrics-ca.key metrics-ca.crt metrics-ca-config.json
$ make certs
```
##### 部署 prometheus 自定义指标 api 适配器
```
$ kubectl create -f ./custom-metrics-api
```
##### 列出 prometheus 提供的自定义指标
```
$ kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/"  | jq .
```
##### 查看自定义指标值
```
$kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1/namespaces/default/pods/*/http_requests" | jq .
{
  "kind": "MetricValueList",
  "apiVersion": "custom.metrics.k8s.io/v1beta1",
  "metadata": {
    "selfLink": "/apis/custom.metrics.k8s.io/v1beta1/namespaces/default/pods/%2A/http_requests"
  },
  "items": [
    {
      "describedObject": {
        "kind": "Pod",
        "namespace": "default",
        "name": "podinfo-6b86c8ccc9-kv5g9",
        "apiVersion": "/__internal"
      },
      "metricName": "http_requests",
      "timestamp": "2018-01-10T16:49:07Z",
      "value": "901m"
    },
    {
      "describedObject": {
        "kind": "Pod",
        "namespace": "default",
        "name": "podinfo-6b86c8ccc9-nm7bl",
        "apiVersion": "/__internal"
      },
      "metricName": "http_requests",
      "timestamp": "2018-01-10T16:49:07Z",
      "value": "898m"
    }
  ]
}
```
##### 基于自定义指标创建 hpa
```
$ kubectl create -f ./podinfo/podinfo-hpa-custom.yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: podinfo
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: podinfo
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Pods
    pods:
        metric:
          name: http_requests
        target:
          type: AverageValue
          averageValue: 10
```
##### 查询 hpa 服务
```
$ kubectl get hpa
NAME      REFERENCE            TARGETS     MINPODS   MAXPODS   REPLICAS   AGE
podinfo   Deployment/podinfo   899m / 10   2         10        2          1m
```
##### 通过压力测试，测试 hpa 是否扩缩容
```
#install hey
go get -u github.com/rakyll/hey
#do 10K requests rate limited at 25 QPS
hey -n 10000 -q 5 -c 5 http://<K8S-IP>:31198/healthz
```
##### 查看 hpa 服务
```
//此处已经更新 pod 副本数量
kubectl describe hpa
Name:                       podinfo
Namespace:                  default
Reference:                  Deployment/podinfo
Metrics:                    ( current / target )
  "http_requests" on pods:  9059m / 10
Min replicas:               2
Max replicas:               10

Events:
  Type    Reason             Age   From                       Message
  ----    ------             ----  ----                       -------
  Normal  SuccessfulRescale  2m    horizontal-pod-autoscaler  New size: 3; reason: pods metric http_requests above target
```
#### 上面通过实际操作知道 hpa 如何使用，现在分析源码
##### 初始化
````
//文件位置:cmd/kube-controller-manager/app/controllermanager.go
func NewControllerInitializers(loopMode ControllerLoopMode) map[string]InitFunc {
	...
	controllers["horizontalpodautoscaling"] = startHPAController
    ...
///HPA Controller 和其他的 Controller 一样，都在 NewControllerInitializers 方法中进行注册，然后通过 startHPAController  来启动。    
````
##### startHPAController
```
//文件位置：cmd/kube-controller-manager/app/autoscaling.go
func startHPAController(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
    ...
	return startHPAControllerWithRESTClient(ctx, controllerContext)
}

func startHPAControllerWithRESTClient(ctx context.Context, controllerContext ControllerContext) (controller.Interface, bool, error) {
	//通过上下文构建一个 clientConfig
	clientConfig := controllerContext.ClientBuilder.ConfigOrDie("horizontal-pod-autoscaler")
	//生成 hpa 客户端
	hpaClient := controllerContext.ClientBuilder.ClientOrDie("horizontal-pod-autoscaler")
	apiVersionsGetter := custom_metrics.NewAvailableAPIsGetter(hpaClient.Discovery())
	// invalidate the discovery information roughly once per resync interval our API
	// information is *at most* two resync intervals old.
	go custom_metrics.PeriodicallyInvalidate(
		apiVersionsGetter,
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerSyncPeriod.Duration,
		ctx.Done())
	// 生成 metrics 客户端
	metricsClient := metrics.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(clientConfig),
		custom_metrics.NewForConfig(clientConfig, controllerContext.RESTMapper, apiVersionsGetter),
		external_metrics.NewForConfigOrDie(clientConfig),
	)
	return startHPAControllerWithMetricsClient(ctx, controllerContext, metricsClient)
}

func startHPAControllerWithMetricsClient(ctx context.Context, controllerContext ControllerContext, metricsClient metrics.MetricsClient) (controller.Interface, bool, error) {
	hpaClient := controllerContext.ClientBuilder.ClientOrDie("horizontal-pod-autoscaler")
	hpaClientConfig := controllerContext.ClientBuilder.ConfigOrDie("horizontal-pod-autoscaler")

	// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
	// so we want to re-fetch every time when we actually ask for it
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(hpaClient.Discovery())
	scaleClient, err := scale.NewForConfig(hpaClientConfig, controllerContext.RESTMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return nil, false, err
	}
    //初始化，初始化一些 client
	go podautoscaler.NewHorizontalController(
		hpaClient.CoreV1(),
		scaleClient,
		hpaClient.AutoscalingV2(),
		controllerContext.RESTMapper,
		metricsClient,
		controllerContext.InformerFactory.Autoscaling().V2().HorizontalPodAutoscalers(),
		controllerContext.InformerFactory.Core().V1().Pods(),
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerSyncPeriod.Duration,
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerDownscaleStabilizationWindow.Duration,
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerTolerance,
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerCPUInitializationPeriod.Duration,
		controllerContext.ComponentConfig.HPAController.HorizontalPodAutoscalerInitialReadinessDelay.Duration,
    //Run 里面执行 worker    
	).Run(ctx)
	return nil, true, nil
}
```
##### Run
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
// Run begins watching and syncing.
func (a *HorizontalController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	defer a.queue.ShutDown()

	klog.Infof("Starting HPA controller")
	defer klog.Infof("Shutting down HPA controller")

	if !cache.WaitForNamedCacheSync("HPA", ctx.Done(), a.hpaListerSynced, a.podListerSynced) {
		return
	}

	// start a single worker (we may wish to start more in the future)
	//启动异步线程，每秒执行一次，主程序在worker里面执行
	go wait.UntilWithContext(ctx, a.worker, time.Second)
    //等待有上下文的退出
	<-ctx.Done()
}
```
##### worker
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
func (a *HorizontalController) worker(ctx context.Context) {
    //一直循环处理queue里数据
	for a.processNextWorkItem(ctx) {
	}
	klog.Infof("horizontal pod autoscaler controller worker shutting down")
}

func (a *HorizontalController) processNextWorkItem(ctx context.Context) bool {
	key, quit := a.queue.Get()
	if quit {
		return false
	}
	defer a.queue.Done(key)
    //queue里面是string类型的字符串，例如namespaces/podinfo，这边把key传到reconcileKey函数执行
	deleted, err := a.reconcileKey(ctx, key.(string))
	...
}
```
##### reconcileKey
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
func (a *HorizontalController) reconcileKey(ctx context.Context, key string) (deleted bool, err error) {
	//获取到namespace，name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return true, err
	}
	//通过informer 从缓存里读到 hpa 数据
	hpa, err := a.hpaLister.HorizontalPodAutoscalers(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.Infof("Horizontal Pod Autoscaler %s has been deleted in %s", name, namespace)
		delete(a.recommendations, key)
		delete(a.scaleUpEvents, key)
		delete(a.scaleDownEvents, key)
		return true, nil
	}
	if err != nil {
		return false, err
	}
	//如果 get 到数据，就执行具体扩缩容任务,reconcileAutoscaler 函数真正的执行了 update
	return false, a.reconcileAutoscaler(ctx, hpa, key)
}
```
##### reconcileAutoscaler
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
func (a *HorizontalController) reconcileAutoscaler(ctx context.Context, hpaShared *autoscalingv2.HorizontalPodAutoscaler, key string) error {
	// make a copy so that we never mutate the shared informer cache (conversion can mutate the object)
	//制作副本，因为数据是从缓存里拿的，这边不对缓存数据直接修改
	hpa := hpaShared.DeepCopy()
	hpaStatusOriginal := hpa.Status.DeepCopy()
	//reference = kind/namespaces/name 此处获取的值为 hpa 扩缩容对象值
	reference := fmt.Sprintf("%s/%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)
	//targetGV hpa spec 里的 apiversion，比如 apps/v1 变成 结构体两个值 apps、v1
	targetGV, err := schema.ParseGroupVersion(hpa.Spec.ScaleTargetRef.APIVersion)
	//err 不等于空
	if err != nil {
		// event 记录事件
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetScale", err.Error())
		//把一些值给 hpa.Status.Conditions 结构体
		setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		//判断复制的hpa跟指针地址里面的hpa数据是否一致，如果不一致更新复制的hpa
		a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa)
		return fmt.Errorf("invalid API version in scale target reference: %v", err)
	}
	//通过Group，kind生成结构体给后面的 client 调用
	targetGK := schema.GroupKind{
		Group: targetGV.Group,
		Kind:  hpa.Spec.ScaleTargetRef.Kind,
	}
	//RESTMapping中有Resource名称，GVK，Scope，Convertor，Accessor等和GVR有关的信息，具体作用不是很清楚
	mappings, err := a.mapper.RESTMappings(targetGK)
	if err != nil {
		//如果 err 不等与空，event 记录事件
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetScale", err.Error())
		//更新status
		setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa)
		return fmt.Errorf("unable to determine resource for scale target reference: %v", err)
	}
	//获取 scale 数据，这里是调用client-go 客户端 Get 方法，传进去的值有 namespace，name，GKV，主要想获取的数据有 replicas，selector
	scale, targetGR, err := a.scaleForResourceMappings(ctx, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name, mappings)
	if err != nil {
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedGetScale", err.Error())
		setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionFalse, "FailedGetScale", "the HPA controller was unable to get the target's current scale: %v", err)
		a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa)
		return fmt.Errorf("failed to query scale subresource for %s: %v", reference, err)
	}
	setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionTrue, "SucceededGetScale", "the HPA controller was able to get the target's current scale")
	currentReplicas := scale.Spec.Replicas
	a.recordInitialRecommendation(currentReplicas, key)

	var (
		metricStatuses        []autoscalingv2.MetricStatus
		metricDesiredReplicas int32
		metricName            string
	)
	//调整数量
	desiredReplicas := int32(0)
	//调整原因
	rescaleReason := ""

	var minReplicas int32

	if hpa.Spec.MinReplicas != nil {
		//如果hpa.Spec.MinReplicas 不等于空，minReplicas = hpa.Spec.MinReplicas
		minReplicas = *hpa.Spec.MinReplicas
	} else {
		//默认为1
		// Default value
		minReplicas = 1
	}
	//判断是否需要重新调整的开关
	rescale := true
	//看hpa现在的replicas == 0 与 最小的不等于0
	if scale.Spec.Replicas == 0 && minReplicas != 0 {
		// Autoscaling is disabled for this resource
		//调整数量为0
		desiredReplicas = 0
		//开关
		rescale = false
        //把值放进 status
		setCondition(hpa, autoscalingv2.ScalingActive, v1.ConditionFalse, "ScalingDisabled", "scaling is disabled since the replica count of the target is zero")
	} else if currentReplicas > hpa.Spec.MaxReplicas { //现存数量大于最大数量
		rescaleReason = "Current number of replicas above Spec.MaxReplicas" //说明
		desiredReplicas = hpa.Spec.MaxReplicas //调整数量为最大数量
	} else if currentReplicas < minReplicas { //现存数量小于最小数量
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = minReplicas //调整数量为最小数量
	} else {
		var metricTimestamp time.Time
		//replicas不超过最大，不低于最小，通过计算判断 metricDesiredReplicas 调整数量
		metricDesiredReplicas, metricName, metricStatuses, metricTimestamp, err = a.computeReplicasForMetrics(ctx, hpa, scale, hpa.Spec.Metrics)
		if err != nil {
			//更新status状态
			a.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			a.eventRecorder.Event(hpa, v1.EventTypeWarning, "FailedComputeMetricsReplicas", err.Error())
			return fmt.Errorf("failed to compute desired number of replicas based on listed metrics for %s: %v", reference, err)
		}

		klog.V(4).Infof("proposing %v desired replicas (based on %s from %s) for %s", metricDesiredReplicas, metricName, metricTimestamp, reference)

		rescaleMetric := ""
		if metricDesiredReplicas > desiredReplicas {
			desiredReplicas = metricDesiredReplicas
			rescaleMetric = metricName
		}
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}
		//可以在扩缩容的时候指定一个稳定窗口，以防止缩放目标中的副本数量出现波动
		if hpa.Spec.Behavior == nil {
			desiredReplicas = a.normalizeDesiredReplicas(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		} else {
			desiredReplicas = a.normalizeDesiredReplicasWithBehaviors(hpa, key, currentReplicas, desiredReplicas, minReplicas)
		}
		//改变rescale的布尔值， 现存状态与期望状态一致 为 false
		rescale = desiredReplicas != currentReplicas
	}

	if rescale {
		//改变现存状态数量，update
		scale.Spec.Replicas = desiredReplicas
        //更新
		_, err = a.scaleNamespacer.Scales(hpa.Namespace).Update(ctx, targetGR, scale, metav1.UpdateOptions{})
		if err != nil {
			a.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedRescale", "New size: %d; reason: %s; error: %v", desiredReplicas, rescaleReason, err.Error())
			setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionFalse, "FailedUpdateScale", "the HPA controller was unable to update the target scale: %v", err)
			a.setCurrentReplicasInStatus(hpa, currentReplicas)
			if err := a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa); err != nil {
				utilruntime.HandleError(err)
			}
			return fmt.Errorf("failed to rescale %s: %v", reference, err)
		}
        //更新status
		setCondition(hpa, autoscalingv2.AbleToScale, v1.ConditionTrue, "SucceededRescale", "the HPA controller was able to update the target scale to %d", desiredReplicas)
		a.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "SuccessfulRescale", "New size: %d; reason: %s", desiredReplicas, rescaleReason)
		a.storeScaleEvent(hpa.Spec.Behavior, key, currentReplicas, desiredReplicas)
		klog.Infof("Successful rescale of %s, old size: %d, new size: %d, reason: %s",
			hpa.Name, currentReplicas, desiredReplicas, rescaleReason)
	} else {
		klog.V(4).Infof("decided not to scale %s to %v (last scale time was %s)", reference, desiredReplicas, hpa.Status.LastScaleTime)
		desiredReplicas = currentReplicas
	}
	//更新 hpa 状态
	a.setStatus(hpa, currentReplicas, desiredReplicas, metricStatuses, rescale)
	return a.updateStatusIfNeeded(ctx, hpaStatusOriginal, hpa)
}
//以上这一段代码主要为获取 hpa 数据，通过 hpa 需要扩容的对象数据获取 scale 数据，通过判断 scale 里 replicas 数量等数据，执行获取期望值函数，获取到期望值并更新 scale、hpa。
//scale 的 replicas > 最大值或 < 最小值就把 replicas 值更新成最大或最小，replicas = 0 就返回，这些都好理解。
//主要是有一个 replicas 在 最大与最小区间的计算
//代码里有一些核心函数，接着分析
```
##### scaleForResourceMappings
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
func (a *HorizontalController) scaleForResourceMappings(ctx context.Context, namespace, name string, mappings []*apimeta.RESTMapping) (*autoscalingv1.Scale, schema.GroupResource, error) {
	var firstErr error
    //mapping 主要是获取对于的GR
	for i, mapping := range mappings {
		targetGR := mapping.Resource.GroupResource()
		//通过 get 方法获取 scale 数据
		scale, err := a.scaleNamespacer.Scales(namespace).Get(ctx, targetGR, name, metav1.GetOptions{})
		if err == nil {
			//erro 等于 nil 返回
			return scale, targetGR, nil
		}
    ...
}
```
##### computeReplicasForMetrics
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
// computeReplicasForMetrics computes the desired number of replicas for the metric specifications listed in the HPA,
// returning the maximum  of the computed replica counts, a description of the associated metric, and the statuses of
// all metrics computed.
func (a *HorizontalController) computeReplicasForMetrics(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler, scale *autoscalingv1.Scale,
	metricSpecs []autoscalingv2.MetricSpec) (replicas int32, metric string, statuses []autoscalingv2.MetricStatus, timestamp time.Time, err error) {
	//scale 必须要有 selector, selector 是一个标签
	if scale.Status.Selector == "" {
		errMsg := "selector is required"
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "SelectorRequired", errMsg)
		setCondition(hpa, autoscalingv2.ScalingActive, v1.ConditionFalse, "InvalidSelector", "the HPA target's scale is missing a selector")
		return 0, "", nil, time.Time{}, fmt.Errorf(errMsg)
	}

	selector, err := labels.Parse(scale.Status.Selector)
	if err != nil {
		errMsg := fmt.Sprintf("couldn't convert selector into a corresponding internal selector object: %v", err)
		a.eventRecorder.Event(hpa, v1.EventTypeWarning, "InvalidSelector", errMsg)
		setCondition(hpa, autoscalingv2.ScalingActive, v1.ConditionFalse, "InvalidSelector", errMsg)
		return 0, "", nil, time.Time{}, fmt.Errorf(errMsg)
	}

	specReplicas := scale.Spec.Replicas
	statusReplicas := scale.Status.Replicas
	statuses = make([]autoscalingv2.MetricStatus, len(metricSpecs))

	invalidMetricsCount := 0
	var invalidMetricError error
	var invalidMetricCondition autoscalingv2.HorizontalPodAutoscalerCondition
	//metricsSpecs是一个数组，hpa 里的 spec 下可以有多个对象，所以需要 for 循环
	for i, metricSpec := range metricSpecs {
		//根据type类型计算需要扩缩容的数量
        //之前的操作都是获取数据，判断err，判断是否需要扩缩容，但是没有计算指标判断出需要扩所容的数量，computeReplicasForMetric 计算扩缩容数量
		replicaCountProposal, metricNameProposal, timestampProposal, condition, err := a.computeReplicasForMetric(ctx, hpa, metricSpec, specReplicas, statusReplicas, selector, &statuses[i])

		if err != nil {
			if invalidMetricsCount <= 0 {
				invalidMetricCondition = condition
				invalidMetricError = err
			}
			invalidMetricsCount++
		}
        //记录最大的需要扩缩容的数量
		if err == nil && (replicas == 0 || replicaCountProposal > replicas) {
			timestamp = timestampProposal
			replicas = replicaCountProposal
			metric = metricNameProposal
		}
	}
	if invalidMetricsCount >= len(metricSpecs) || (invalidMetricsCount > 0 && replicas < specReplicas) {
		setCondition(hpa, invalidMetricCondition.Type, invalidMetricCondition.Status, invalidMetricCondition.Reason, invalidMetricCondition.Message)
		return 0, "", statuses, time.Time{}, fmt.Errorf("invalid metrics (%v invalid out of %v), first error is: %v", invalidMetricsCount, len(metricSpecs), invalidMetricError)
	}
	setCondition(hpa, autoscalingv2.ScalingActive, v1.ConditionTrue, "ValidMetricFound", "the HPA was able to successfully calculate a replica count from %s", metric)
	return replicas, metric, statuses, timestamp, nil
}
```
##### computeReplicasForMetric
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
// Computes the desired number of replicas for a specific hpa and metric specification,
// returning the metric status and a proposed condition to be set on the HPA object.
func (a *HorizontalController) computeReplicasForMetric(ctx context.Context, hpa *autoscalingv2.HorizontalPodAutoscaler, spec autoscalingv2.MetricSpec,
	specReplicas, statusReplicas int32, selector labels.Selector, status *autoscalingv2.MetricStatus) (replicaCountProposal int32, metricNameProposal string,
	timestampProposal time.Time, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	//计算Type类型
    //hpa.spec 里需要的对象有可能是 ingress、有可能是 pod、有可能是外部服务，所以这里要对应对象类型进行获取相应数据，这里举例为 pod 继续分析
	switch spec.Type {
	//如果type 是 object 对象，比如 ingress
	case autoscalingv2.ObjectMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Object.Metric.Selector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetObjectMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForObjectMetric(specReplicas, statusReplicas, spec, hpa, selector, status, metricSelector)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get object metric value: %v", err)
		}
	//如果type是pod
	case autoscalingv2.PodsMetricSourceType:
		//判断有没有标签，没有标签，记录statue 错误状态，返回统计更新数量0等
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Pods.Metric.Selector)
		if err != nil {
			condition := a.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
        //computeStatusForPodsMetric 就是处理具体指标数据并计算
		replicaCountProposal, timestampProposal, metricNameProposal, condition, err = a.computeStatusForPodsMetric(specReplicas, spec, hpa, selector, status, metricSelector)
		if err != nil {
			return 0, "", time.Time{}, condition, fmt.Errorf("failed to get pods metric value: %v", err)
		}
	case autoscalingv2.ResourceMetricSourceType:
		...
	case autoscalingv2.ContainerResourceMetricSourceType:
		...
	case autoscalingv2.ExternalMetricSourceType:
		...
	default:
		errMsg := fmt.Sprintf("unknown metric source type %q", string(spec.Type))
		err = fmt.Errorf(errMsg)
		condition := a.getUnableComputeReplicaCountCondition(hpa, "InvalidMetricSourceType", err)
		return 0, "", time.Time{}, condition, err
	}
	return replicaCountProposal, metricNameProposal, timestampProposal, autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
}
```
##### computeStatusForPodsMetric
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
// computeStatusForPodsMetric computes the desired number of replicas for the specified metric of type PodsMetricSourceType.
func (a *HorizontalController) computeStatusForPodsMetric(currentReplicas int32, metricSpec autoscalingv2.MetricSpec, hpa *autoscalingv2.HorizontalPodAutoscaler, selector labels.Selector, status *autoscalingv2.MetricStatus, metricSelector labels.Selector) (replicaCountProposal int32, timestampProposal time.Time, metricNameProposal string, condition autoscalingv2.HorizontalPodAutoscalerCondition, err error) {
	//GetMetricReplicas 里用了 metrics client 获取数据，并计算
	replicaCountProposal, utilizationProposal, timestampProposal, err := a.replicaCalc.GetMetricReplicas(currentReplicas, metricSpec.Pods.Target.AverageValue.MilliValue(), metricSpec.Pods.Metric.Name, hpa.Namespace, selector, metricSelector)
	if err != nil {
		condition = a.getUnableComputeReplicaCountCondition(hpa, "FailedGetPodsMetric", err)
		return 0, timestampProposal, "", condition, err
	}
	*status = autoscalingv2.MetricStatus{
		Type: autoscalingv2.PodsMetricSourceType,
		Pods: &autoscalingv2.PodsMetricStatus{
			Metric: autoscalingv2.MetricIdentifier{
				Name:     metricSpec.Pods.Metric.Name,
				Selector: metricSpec.Pods.Metric.Selector,
			},
			Current: autoscalingv2.MetricValueStatus{
				AverageValue: resource.NewMilliQuantity(utilizationProposal, resource.DecimalSI),
			},
		},
	}

	return replicaCountProposal, timestampProposal, fmt.Sprintf("pods metric %s", metricSpec.Pods.Metric.Name), autoscalingv2.HorizontalPodAutoscalerCondition{}, nil
}
```
##### GetMetricReplicas
```
//文件位置：pkg/controller/podautoscaler/horizontal.go
// GetMetricReplicas calculates the desired replica count based on a target metric utilization
// (as a milli-value) for pods matching the given selector in the given namespace, and the
// current replica count
func (c *ReplicaCalculator) GetMetricReplicas(currentReplicas int32, targetUtilization int64, metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (replicaCount int32, utilization int64, timestamp time.Time, err error) {
	//获取对于度量数据，次数获取度量数据最终的方法是 get URL
	metrics, timestamp, err := c.metricsClient.GetRawMetric(metricName, namespace, selector, metricSelector)
	if err != nil {
		return 0, 0, time.Time{}, fmt.Errorf("unable to get metric %s: %v", metricName, err)
	}
	//结合度量数据与目标期望来计算希望扩缩容的数量是多少
	replicaCount, utilization, err = c.calcPlainMetricReplicas(metrics, currentReplicas, targetUtilization, namespace, selector, v1.ResourceName(""))
	return replicaCount, utilization, timestamp, err
}
```
##### calcPlainMetricReplicas
```
// calcPlainMetricReplicas calculates the desired replicas for plain (i.e. non-utilization percentage) metrics.
func (c *ReplicaCalculator) calcPlainMetricReplicas(metrics metricsclient.PodMetricsInfo, currentReplicas int32, targetUtilization int64, namespace string, selector labels.Selector, resource v1.ResourceName) (replicaCount int32, utilization int64, err error) {
	//通过 pod client 获取命名空间下，指定标签的一个列表
	podList, err := c.podLister.Pods(namespace).List(selector)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}
	//list 等于 0 不调度
	if len(podList) == 0 {
		return 0, 0, fmt.Errorf("no pods returned by selector while calculating replica count")
	}
	//将pod分成三类进行统计，得到ready的pod数量、ignored Pod集合、missing Pod集合
	readyPodCount, unreadyPods, missingPods, ignoredPods := groupPods(podList, metrics, resource, c.cpuInitializationPeriod, c.delayOfInitialReadinessStatus)
	//在度量的数据里移除ignored Pods集合的数据
	removeMetricsForPods(metrics, ignoredPods)
	//在度量的数据里移除unready Pods集合的数据
	removeMetricsForPods(metrics, unreadyPods)

	//metrics get 到指标的数量删除 没准备的，忽略的，等于 0，返回期望状态0
	if len(metrics) == 0 {
		return 0, 0, fmt.Errorf("did not receive metrics for any ready pods")
	}
    //没准备的，忽略的都删除了，剩下来的计算
	//获取资源使用率，每个pod使用率 for 循环相加，总使用率 / 每个pod，指标 / 刚刚获得的每台节点平均使用率，
	usageRatio, utilization := metricsclient.GetMetricUtilizationRatio(metrics, targetUtilization)
    //扩容扩容数量大于0，没准备的pod>0，rebalanceIgnored 为 true
	rebalanceIgnored := len(unreadyPods) > 0 && usageRatio > 1.0

	if !rebalanceIgnored && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= c.tolerance {
            //如果判断出来的指标值小于容忍，返回当前副本
			// return the current replicas if the change would be too small
			return currentReplicas, utilization, nil
		}
		//如果没有 unready 或 missing 的pod，那么使用 usageRatio*readyPodCount计算需要扩缩容数量
		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount))), utilization, nil
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			//如果是缩容，那么将 missing pod 使用率设置为目标资源使用率
			// on a scale-down, treat missing pods as using 100% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: targetUtilization}
			}
		} else {
			//如果是扩容，那么将 missing pod 使用率设置为 0
			// on a scale-up, treat missing pods as using 0% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: 0}
			}
		}
	}

	if rebalanceIgnored {
		// on a scale-up, treat unready pods as using 0% of the resource request
		// 将 unready pods 使用率设置为 0
		for podName := range unreadyPods {
			metrics[podName] = metricsclient.PodMetric{Value: 0}
		}
	}

	// re-run the utilization calculation with our new numbers
	//重新计算资源使用率，看看是不是还要扩容
	newUsageRatio, _ := metricsclient.GetMetricUtilizationRatio(metrics, targetUtilization)

	if math.Abs(1.0-newUsageRatio) <= c.tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas, utilization, nil
	}

	newReplicas := int32(math.Ceil(newUsageRatio * float64(len(metrics))))
	if (newUsageRatio < 1.0 && newReplicas > currentReplicas) || (newUsageRatio > 1.0 && newReplicas < currentReplicas) {
		// return the current replicas if the change of metrics length would cause a change in scale direction
		return currentReplicas, utilization, nil
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return newReplicas, utilization, nil
}
```
##### metrics 指标
```
//文件位置：pkg/controller/podautoscaler/replica_calcalator
//上述代码中，通过metrics 通过 get 取 url 数据，那么 url 如何形成的？
func (c *ReplicaCalculator) GetMetricReplicas(currentReplicas int32, targetUtilization int64, metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (replicaCount int32, utilization int64, timestamp time.Time, err error) {
	//获取对于度量数据
	metrics, timestamp, err := c.metricsClient.GetRawMetric(metricName, namespace, selector, metricSelector)
    ...
}

func (c *customMetricsClient) GetRawMetric(metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (PodMetricsInfo, time.Time, error) {
    //通过GetForObjects方法
	metrics, err := c.client.NamespacedMetrics(namespace).GetForObjects(schema.GroupKind{Kind: "Pod"}, selector, metricName, metricSelector)
	...
}

func (m *namespacedMetrics) GetForObjects(groupKind schema.GroupKind, selector labels.Selector, metricName string, metricSelector labels.Selector) (*v1beta2.MetricValueList, error) {
	...
    //此处进行了字符串拼接与获取数据
	result := m.client.client.Get().
		Resource(resourceName).
		Namespace(m.namespace).
		Name(v1beta1.AllObjects).
		SubResource(metricName).
		VersionedParams(params, scheme.ParameterCodec).
		Do(context.TODO()) //Do函数里执行 http 服务获取 URL 数据
    ...    
}

// URL returns the current working URL.
func (r *Request) URL() *url.URL {
	p := r.pathPrefix
    //通过 join 拼接字符串
	if r.namespaceSet && len(r.namespace) > 0 {
		p = path.Join(p, "namespaces", r.namespace)
	}
	if len(r.resource) != 0 {
		p = path.Join(p, strings.ToLower(r.resource))
	}
	// Join trims trailing slashes, so preserve r.pathPrefix's trailing slash for backwards compatibility if nothing was changed
	if len(r.resourceName) != 0 || len(r.subpath) != 0 || len(r.subresource) != 0 {
		p = path.Join(p, r.resourceName, r.subresource, r.subpath)
	}

	finalURL := &url.URL{}
	if r.c.base != nil {
		*finalURL = *r.c.base
	}
	finalURL.Path = p

	query := url.Values{}
	for key, values := range r.params {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	// timeout is handled specially here.
	if r.timeout != 0 {
		query.Set("timeout", r.timeout.String())
	}
	finalURL.RawQuery = query.Encode()
	return finalURL
}
```
##### 到此HPA代码部分暂时分析完毕
```
//之前通过kubectl get --raw="/apis/custom.metrics.k8s.io/v1beta1/" | jq 就可以获取很多指标，
//那么自定义指标数据哪来的呢？为什么要 apis 后面跟着 custom.metrics.k8s.io/v1beta1
//这是通过apiservice创建了一个api
$ kubectl get apiservice
NAME                                   SERVICE                               AVAILABLE   AGE
...
v1beta1.certificates.k8s.io            Local                                 True        21d
v1beta1.coordination.k8s.io            Local                                 True        21d
v1beta1.custom.metrics.k8s.io          monitoring/custom-metrics-apiserver   True        7h42m
v1beta1.discovery.k8s.io               Local                                 True        21d
v1beta1.events.k8s.io                  Local                                 True        21d
v1beta1.extensions                     Local                                 True        21d
v1beta1.metrics.k8s.io                 kube-system/metrics-server            True        7h55m
v1beta1.networking.k8s.io              Local                                 True        21d
v1beta1.node.k8s.io                    Local                                 True        21d
...
//通过 -o yaml 看一下具体格式，此处是社区项目 k8s-prom-hpa/custom-metrics-api/custom-metrics-apiservice.yaml 文件生成
$ kubectl get apiservice v1beta1.custom.metrics.k8s.io -o yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apiregistration.k8s.io/v1beta1","kind":"APIService","metadata":{"annotations":{},"name":"v1beta1.custom.metrics.k8s.io"},"spec":{"group":"custom.metrics.k8s.io","groupPriorityMinimum":100,"insecureSkipTLSVerify":true,"service":{"name":"custom-metrics-apiserver","namespace":"monitoring"},"version":"v1beta1","versionPriority":100}}
  creationTimestamp: "2021-12-22T03:46:10Z"
  name: v1beta1.custom.metrics.k8s.io
  resourceVersion: "5405152"
  selfLink: /apis/apiregistration.k8s.io/v1/apiservices/v1beta1.custom.metrics.k8s.io
  uid: 1c374256-6dd9-4893-bbeb-cc808df807ec
spec:
  group: custom.metrics.k8s.io
  groupPriorityMinimum: 100
  insecureSkipTLSVerify: true
  //这边定义了service，就是 svc，访问 custom.metrics.k8s.io/v1beta1 都会转到 custom-metrics-apiserver.monitoring.svc.cluster.local:443 上
  service:
    name: custom-metrics-apiserver
    namespace: monitoring
    port: 443
  version: v1beta1
  versionPriority: 100
status:
  conditions:
  - lastTransitionTime: "2021-12-22T03:47:54Z"
    message: all checks passed
    reason: Passed
    status: "True"
    type: Available
//如果新建了 api，没有服务，使用 kubectl 来获得你的资源。 它应该返回 "找不到资源"。此消息表示一切正常，但你目前还没有创建该资源类型的对象
```
##### 自定义指标需要什么条件？
```
//自定义指标需要什么条件吗？
//我们可以打开prometheus 页面里面的 target 进行查看，发现kubernetes-pods只能检测到一些，发现所有检测到的数据都有一个共通点都有 /metrics 后缀的 url，判断出自定义指标需要有 /metrics 文件
//访问 pod 的 metrics curl 进行查看
$ curl 10.254.181.76:9898/metrics
# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 7.1477e-05
go_gc_duration_seconds{quantile="0.25"} 0.000122777
go_gc_duration_seconds{quantile="0.5"} 0.000143479
go_gc_duration_seconds{quantile="0.75"} 0.000228246
go_gc_duration_seconds{quantile="1"} 0.001736077
go_gc_duration_seconds_sum 0.162577005
go_gc_duration_seconds_count 672
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 13
# HELP go_memstats_alloc_bytes Number of bytes allocated and still in use.
# TYPE go_memstats_alloc_bytes gauge
go_memstats_alloc_bytes 1.540504e+06
# HELP go_memstats_alloc_bytes_total Total number of bytes allocated, even if freed.
# TYPE go_memstats_alloc_bytes_total counter
go_memstats_alloc_bytes_total 2.002539888e+09
# HELP go_memstats_buck_hash_sys_bytes Number of bytes used by the profiling bucket hash table.
# TYPE go_memstats_buck_hash_sys_bytes gauge
go_memstats_buck_hash_sys_bytes 1.483493e+06
# HELP go_memstats_frees_total Total number of frees.
# TYPE go_memstats_frees_total counter
go_memstats_frees_total 5.465523e+06
# HELP go_memstats_gc_sys_bytes Number of bytes used for garbage collection system metadata.
# TYPE go_memstats_gc_sys_bytes gauge
....
//判断出 pod 有哪些值可以做自定义指标
```
##### 新增自定义指标
```
//前面通过 apiservice 生成了 api 并指向 svc，svc具体为 monitoring 命名空间下 custom-metrics-apiserver pod
//这个控制器是社区的 prometheus-adapter 项目
//这个 pod 挂载了一个 configmap，我们可以通过新增 configmap 里文件内容重建新的 pod，生成自定义指标
$ kubectl edit configmap adapter-config -n monitoring
apiVersion: v1
data:
  config.yaml: |
    rules:
    //更具格式照抄一个自定义指标
    //http_requests_total， metrics url 必须能查到
    - seriesQuery: 'http_requests_total{kubernetes_namespace!="",kubernetes_pod_name!=""}'
      resources:
        overrides:
          kubernetes_namespace: {resource: "namespace"}
          kubernetes_pod_name: {resource: "pod"}
      name:
        matches: "^(.*)_total"
        //自定义指标最终效果 test_jixingxing
        as: "test_jixingxing"
      //计算2分钟内 http_requests_total 访问量
      //详细信息访问社区 doc
      metricsQuery: 'sum(rate(<<.Series>>{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>)'
    - seriesQuery: '{__name__=~"^container_.*",container_name!="POD",namespace!="",pod_name!=""}'
      seriesFilters: []
      resources:
        overrides:
          namespace:
            resource: namespace
          pod_name:
            resource: pod
      name:
        matches: ^container_(.*)_seconds_total$
        as: ""
      metricsQuery: sum(rate(<<.Series>>{<<.LabelMatchers>>,container_name!="POD"}[1m])) by (<<.GroupBy>>)
    ...

$ kubectl get --raw="/apis/custom.metrics.k8s.io/v1beta1/namespaces/default/pod/*/test_jixingxing" | jq
{
  "kind": "MetricValueList",
  "apiVersion": "custom.metrics.k8s.io/v1beta1",
  "metadata": {
    "selfLink": "/apis/custom.metrics.k8s.io/v1beta1/namespaces/default/pod/%2A/test_jixingxing"
  },
  "items": [
    {
      "describedObject": {
        "kind": "Pod",
        "namespace": "default",
        "name": "podinfo-67dbdd475f-jrb4s",
        "apiVersion": "/v1"
      },
      "metricName": "test_jixingxing",
      "timestamp": "2021-12-22T12:26:18Z",
      "value": "0"
    },
    {
      "describedObject": {
        "kind": "Pod",
        "namespace": "default",
        "name": "podinfo-67dbdd475f-vjkss",
        "apiVersion": "/v1"
      },
      "metricName": "test_jixingxing",
      "timestamp": "2021-12-22T12:26:18Z",
      "value": "0"
    }
  ]
}
```
##### 默认 30 秒同步
```
您可能已经注意到，自动调节程序不会立即对使用高峰做出反应。默认情况下，指标同步每 30 秒发生一次，并且只有在过去 3-5 分钟内没有重新缩放时才会进行缩放。通过这种方式，HPA 可以防止快速执行有冲突的决策，并为 Cluster Autoscaler 提供时间启动。
```
##### 结论
```
并非所有系统都可以仅依靠 CPU/内存使用指标来满足其 SLA，大多数 Web 和移动后端都需要基于每秒请求数进行自动缩放以处理任何流量突发。对于 ETL 应用程序，自动缩放可能由作业队列长度超过某个阈值等触发。通过使用 Prometheus 检测您的应用程序并公开正确的自动缩放指标，您可以微调您的应用程序以更好地处理突发并确保高可用性。
```
