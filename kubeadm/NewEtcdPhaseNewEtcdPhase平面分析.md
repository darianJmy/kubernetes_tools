# NewEtcdPhase 平面分析
###### 分析版本为1.22，代码入口 cmd/kubeadm/app/cmd/init.go
##### NewEtcdPhase平面主要创建 /var/lib/etcd 文件夹、创建 etcd 静态 pod

##### 1、NewEtcd 平面涉及子平面，且只有自身没有 Run 函数，但是执行 newEtcdLocalSubPhase 平面函数
```
	phase := workflow.Phase{
		Name:  "etcd",
		Short: "Generate static Pod manifest file for local etcd",
		Long:  cmdutil.MacroCommandLongDescription,
		Phases: []workflow.Phase{
			newEtcdLocalSubPhase(),
		},
	}
	return phase
```

##### 2、newEtcdLocalSubPhase 平面执行 runEtcdPhaseLocal 函数
```
func newEtcdLocalSubPhase() workflow.Phase {
	phase := workflow.Phase{
		Name:         "local",
		Short:        "Generate the static Pod manifest file for a local, single-node local etcd instance",
		Example:      etcdLocalExample,
		Run:          runEtcdPhaseLocal(),
		InheritFlags: getEtcdPhaseFlags(),
	}
	return phase
}
```

##### 3、newEtcdLocalSubPhase 平面执行 runEtcdPhaseLocal 函数
```
        //判断有没有外部 etcd
		// Add etcd static pod spec only if external etcd is not configured
		if cfg.Etcd.External == nil {
			// creates target folder if doesn't exist already
            //判断 dryrun 是否为 false
			if !data.DryRun() {
				// Create the etcd data directory
                //CreateDataDirectory 函数里调用 os.MkdirAll 函数创建 DataDir，默认为 /var/lib/etcd
				if err := etcdutil.CreateDataDirectory(cfg.Etcd.Local.DataDir); err != nil {
					return err
				}
			} else {
				fmt.Printf("[dryrun] Would ensure that %q directory is present\n", cfg.Etcd.Local.DataDir)
			}
			fmt.Printf("[etcd] Creating static Pod manifest for local etcd in %q\n", data.ManifestDir())
            //创建静态 etcd pod
			if err := etcdphase.CreateLocalEtcdStaticPodManifestFile(data.ManifestDir(), data.PatchesDir(), cfg.NodeRegistration.Name, &cfg.ClusterConfiguration, &cfg.LocalAPIEndpoint, data.DryRun()); err != nil {
				return errors.Wrap(err, "error creating local etcd static pod manifest file")
			}
		} else {
			klog.V(1).Infoln("[etcd] External etcd mode. Skipping the creation of a manifest for local etcd")
		}
```

##### 4、etcd 静态 pod
```
// CreateLocalEtcdStaticPodManifestFile will write local etcd static pod manifest file.
// This function is used by init - when the etcd cluster is empty - or by kubeadm
// upgrade - when the etcd cluster is already up and running (and the --initial-cluster flag have no impact)
func CreateLocalEtcdStaticPodManifestFile(manifestDir, patchesDir string, nodeName string, cfg *kubeadmapi.ClusterConfiguration, endpoint *kubeadmapi.APIEndpoint, isDryRun bool) error {
	if cfg.Etcd.External != nil {
		return errors.New("etcd static pod manifest cannot be generated for cluster using external etcd")
	}

    // 创建静态 pod
	if err := prepareAndWriteEtcdStaticPod(manifestDir, patchesDir, cfg, endpoint, nodeName, []etcdutil.Member{}, isDryRun); err != nil {
		return err
	}

	klog.V(1).Infof("[etcd] wrote Static Pod manifest for a local etcd member to %q\n", kubeadmconstants.GetStaticPodFilepath(kubeadmconstants.Etcd, manifestDir))
	return nil
}

func prepareAndWriteEtcdStaticPod(manifestDir string, patchesDir string, cfg *kubeadmapi.ClusterConfiguration, endpoint *kubeadmapi.APIEndpoint, nodeName string, initialCluster []etcdutil.Member, isDryRun bool) error {
	// gets etcd StaticPodSpec, actualized for the current ClusterConfiguration and the new list of etcd members
    //此处跟 api-server 的静态 pod 结构体相似，实例化一个结构体
	spec := GetEtcdPodSpec(cfg, endpoint, nodeName, initialCluster)

	var usersAndGroups *users.UsersAndGroups
	var err error
	if features.Enabled(cfg.FeatureGates, features.RootlessControlPlane) {
		if isDryRun {
			fmt.Printf("[dryrun] Would create users and groups for %q to run as non-root\n", kubeadmconstants.Etcd)
			fmt.Printf("[dryrun] Would update static pod manifest for %q to run run as non-root\n", kubeadmconstants.Etcd)
		} else {
			usersAndGroups, err = staticpodutil.GetUsersAndGroups()
			if err != nil {
				return errors.Wrap(err, "failed to create users and groups")
			}
			// usersAndGroups is nil on non-linux.
			if usersAndGroups != nil {
				if err := staticpodutil.RunComponentAsNonRoot(kubeadmconstants.Etcd, &spec, usersAndGroups, cfg); err != nil {
					return errors.Wrapf(err, "failed to run component %q as non-root", kubeadmconstants.Etcd)
				}
			}
		}
	}

	// if patchesDir is defined, patch the static Pod manifest
	if patchesDir != "" {
		patchedSpec, err := staticpodutil.PatchStaticPod(&spec, patchesDir, os.Stdout)
		if err != nil {
			return errors.Wrapf(err, "failed to patch static Pod manifest file for %q", kubeadmconstants.Etcd)
		}
		spec = *patchedSpec
	}

	// writes etcd StaticPod to disk
    //此处为创建了静态 pod
	if err := staticpodutil.WriteStaticPodToDisk(kubeadmconstants.Etcd, manifestDir, spec); err != nil {
		return err
	}

	return nil
}
```
