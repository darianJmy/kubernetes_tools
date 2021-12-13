# NewMarkControlPlanePhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewMarkControlPlanePhase平面主要工作打标签加污点

##### 1、NewMarkControlPlane 平面没有涉及子平面，运行内容全在 runMarkControlPlane 函数中
```
	return workflow.Phase{
		Name:    "mark-control-plane",
		Short:   "Mark a node as a control-plane",
		Example: markControlPlaneExample,
		InheritFlags: []string{
			options.NodeName,
			options.CfgPath,
		},
		Run: runMarkControlPlane,
	}
```

##### 2、runMarkControlPlane 函数主要是给 node 加污点
```
// runMarkControlPlane executes mark-control-plane checks logic.
func runMarkControlPlane(c workflow.RunData) error {
	data, ok := c.(InitData)
	if !ok {
		return errors.New("mark-control-plane phase invoked with an invalid data struct")
	}
    //new 一个 client
	client, err := data.Client()
	if err != nil {
		return err
	}
    //init 初始化的值
	nodeRegistration := data.Cfg().NodeRegistration
    //具体执行
	return markcontrolplanephase.MarkControlPlane(client, nodeRegistration.Name, nodeRegistration.Taints)
}

// MarkControlPlane taints the control-plane and sets the control-plane label
func MarkControlPlane(client clientset.Interface, controlPlaneName string, taints []v1.Taint) error {
	// TODO: remove this "deprecated" amend and pass "labelsToAdd" directly:
	// https://github.com/kubernetes/kubeadm/issues/2200
	labels := make([]string, len(labelsToAdd))
	copy(labels, labelsToAdd)
	labels[0] = constants.LabelNodeRoleOldControlPlane + "(deprecated)"

	fmt.Printf("[mark-control-plane] Marking the node %s as control-plane by adding the labels: %v\n",
		controlPlaneName, labels)
    //因为为空直接执行 return 
	if len(taints) > 0 {
		taintStrs := []string{}
		for _, taint := range taints {
			taintStrs = append(taintStrs, taint.ToString())
		}
		fmt.Printf("[mark-control-plane] Marking the node %s as control-plane by adding the taints %v\n", controlPlaneName, taintStrs)
	}
    
	return apiclient.PatchNode(client, controlPlaneName, func(n *v1.Node) {
		markControlPlaneNode(n, taints)
	})
}

// PatchNodeOnce executes patchFn on the node object found by the node name.
// This is a condition function meant to be used with wait.Poll. false, nil
// implies it is safe to try again, an error indicates no more tries should be
// made and true indicates success.
func PatchNodeOnce(client clientset.Interface, nodeName string, patchFn func(*v1.Node), lastError *error) func() (bool, error) {
	return func() (bool, error) {
		// First get the node object
        //函数通过 client get 获取到 node 数据，判断 labels 有没有 kubernetes.io/hostname 有就继续执行
		n, err := client.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			*lastError = err
			return false, nil // retry on any error
		}

		// The node may appear to have no labels at first,
		// so we wait for it to get hostname label.
		if _, found := n.ObjectMeta.Labels[v1.LabelHostname]; !found {
			return false, nil
		}
        //把格式转换后给 oldData 变量
		oldData, err := json.Marshal(n)
		if err != nil {
			*lastError = errors.Wrapf(err, "failed to marshal unmodified node %q into JSON", n.Name)
			return false, *lastError
		}

		// Execute the mutating function
        //执行函数，这边添加了taint、labels
		patchFn(n)

		newData, err := json.Marshal(n)
		if err != nil {
			*lastError = errors.Wrapf(err, "failed to marshal modified node %q into JSON", n.Name)
			return false, *lastError
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
		if err != nil {
			*lastError = errors.Wrap(err, "failed to create two way merge patch")
			return false, *lastError
		}
        //更新 Node
		if _, err := client.CoreV1().Nodes().Patch(context.TODO(), n.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			*lastError = errors.Wrapf(err, "error patching node %q through apiserver", n.Name)
			if apierrors.IsTimeout(err) || apierrors.IsConflict(err) {
				return false, nil
			}
			return false, *lastError
		}

		return true, nil
	}
}

func markControlPlaneNode(n *v1.Node, taints []v1.Taint) {
	for _, label := range labelsToAdd {
		n.ObjectMeta.Labels[label] = ""
	}

	for _, nt := range n.Spec.Taints {
		if !taintExists(nt, taints) {
			taints = append(taints, nt)
		}
	}

	n.Spec.Taints = taints
}
```