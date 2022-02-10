# 通过Gitlab+Jenkins+Harbor+Kubernetes+helm 整套 CICD Demo 测试

- 需要的组件
```
gitlab、jenkins、harbor、kubernetes
```
- gitlab 安装
```
# gitlab 选择用 gitlab 官网提供的 docker run 进行安装。
$ export GITLAB_HOME=/srv/gitlab
$ export GITLAB_HOME=$HOME/gitlab
$ sudo docker run --detach   --hostname 10.10.33.37   --publish 443:443 --publish 80:80 --publish 1022:22   --name gitlab   --restart always   --volume $GITLAB_HOME/config:/etc/gitlab   --volume $GITLAB_HOME/logs:/var/log/gitlab   --volume $GITLAB_HOME/data:/var/opt/gitlab   --shm-size 256m  gitlab/gitlab-ee:latest
# 进入容器查看密码
$ sudo docker exec -it gitlab grep 'Password:' /etc/gitlab/initial_root_password
```
- jenkins 安装
```
# jenkins 部署到 kubernetes 集群，jenkins.yaml 里包含了 ServiceAccount、ClusterRole、ClusterRoleBinding、PersistentVolumeClaim、Deployment、Service
$ kubectl apply -f jenkins.yaml
# 安装时间较长，安装好后进入 pod 查看 jenkins 密码
$ kubectl exec jenkins-7c47df4798-pjfsz cat /var/jenkins_home/secrets/initialAdminPassword
```
- harbor 安装
```
# harbor 安装比较复杂，选择简单方式安装，直接通过 helm 安装
$ helm install harbor bitnami/harbor --set service.nodePorts=30001 -set  global.storageClass=managed-nfs-storage --set service.type=NodePort --set service.tls.enabled=false --set externalURL=http://10.10.33.34:30001
$ echo Username: "admin"
$ echo Password: $(kubectl get secret --namespace default harbor-core-envvars -o jsonpath="{.data.HARBOR_ADMIN_PASSWORD}" | base64 --decode)
```
- jenkins 动态 slaver 模式
```
# jenkins 部署在 kubernetes 中，构建流水线时可以到插件管理处安装所需插件，但是还有一种方式是通过 master + slaver 模式运行，构建时会启动一个 slaver pod，所有构建任务在 slaver pod 里进行
# 
```