package main

import (
	"context"
	"fmt"
	"path"


	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/controller-manager/pkg/clientbuilder"
	//"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

func main() {
	hpaConfig, err := clientcmd.BuildConfigFromFlags("", path.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		panic(err)
	}
	hpaClient, err := kubernetes.NewForConfig(hpaConfig)
	if err != nil {
		panic(err)
	}

	stopper := make(chan struct{})
	defer close(stopper)

	factory := informers.NewSharedInformerFactory(hpaClient, 0)
	hpaInformer := factory.Autoscaling().V1().HorizontalPodAutoscalers()
	informer := hpaInformer.Informer()
	defer runtime.HandleCrash()

	go factory.Start(stopper)

	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Time out waiting for caches to sync"))
		return
	}

	hpaLister := hpaInformer.Lister()

	hpaget, err := hpaLister.HorizontalPodAutoscalers(apiv1.NamespaceDefault).Get("podinfo")
	if err != nil {
		panic(err)
	}

	hpa := hpaget.DeepCopy()
	//hpaStatusOriginal := hpa.Status.DeepCopy()
	//reference := fmt.Sprintf("%s/%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)
	targetGV, err := schema.ParseGroupVersion(hpa.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		panic(err)
	}
	targetGK := schema.GroupKind{
		Group: targetGV.Group,
		Kind:  hpa.Spec.ScaleTargetRef.Kind,
	}

	rootClientBuilder := clientbuilder.SimpleControllerClientBuilder{
		ClientConfig: hpaConfig,
	}
	discoveryClient := rootClientBuilder.DiscoveryClientOrDie("horizontal-pod-autoscaler")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)

	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(hpaClient.Discovery())
	scaleClient, err := scale.NewForConfig(hpaConfig, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		panic(err)
	}

	mappings, err := restMapper.RESTMappings(targetGK)
	if err != nil {
		panic(err)
	}
	targetGR := mappings[0].Resource.GroupResource()
	//获取 scale
	scale, err := scaleClient.Scales("default").Get(context.TODO(),targetGR,"podinfo", v1.GetOptions{})
	if err != nil {
		//erro 不等于 nil 返回
		panic(err)
	}
	fmt.Println(scale.Spec.Replicas)
	fmt.Println(scale.Status)

	gets, err := hpaClient.AutoscalingV1().HorizontalPodAutoscalers("default").Get(context.TODO(),"podinfo",v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	fmt.Println(gets.Spec)

	/*// metricsClient
	metricsClient := metrics.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(clientConfig),
		custom_metrics.NewForConfig(clientConfig, controllerContext.RESTMapper, apiVersionsGetter),
		external_metrics.NewForConfigOrDie(clientConfig),
	)*/
}

