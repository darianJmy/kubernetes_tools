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
$ helm install harbor bitnami/harbor --set service.nodePorts.http=30001 --set  global.storageClass=managed-nfs-storage --set service.type=NodePort --set service.tls.enabled=false --set externalURL=http://10.10.33.34:30001
$ echo Username: "admin"
$ echo Password: $(kubectl get secret --namespace default harbor-core-envvars -o jsonpath="{.data.HARBOR_ADMIN_PASSWORD}" | base64 --decode)
```
- jenkins 配置与动态 slaver
```
# jenkins 部署在 kubernetes 中，构建流水线时可以到插件管理处安装所需插件，但是还有一种方式是通过 master + slaver 模式运行，构建时会启动一个 slaver pod，所有构建任务在 slaver pod 里进行，这样插件也就通过制作 slaver 镜像制作进去，有问题就更新镜像
# jenkins 安装完成后需要进入 pod 查看密码登陆，登陆完成后可以安装系统推荐插件
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-01-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-02-unlock.png)
```
# jenkins 进入页面，选择 系统管理->插件管理->可选插件，安装 kubernetes、gitlab，一个是让 jenkins 可以通过 kubernetes 集群创建动态 slaver、一个是创建项目后通过 webhook 构建项目，安装完插件重启 jenkins 服务
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-03-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-04-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-05-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-06-unlock.png)
```
# jenkins 进入页面，选择 系统管理->系统配置->Cloud，Cloud 一般在最下方，点击 a separate configuration page. 会跳转到另一个界面，选择 Add a new cloud 添加 kubernetes 进去信息以及 slaver 模板信息
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-07-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-08-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-09-unlock.png)
```
# 集群名称随便填写，集群地址选择 https://kubernetes.default.svc.cluster.local，命名空间 default，jenkins 地址填写 http://jenkins.default.svc.cluster.local:8080，
# jenkins 通道填写 jenkins.default.svc.cluster.local:50000，给 pod 取一个名字，命名空间 default，标签列表可以随便写但是创建项目时如果用到 slaver pod 要与此处标签对应，
# 容器取名、选择 slaver 镜像、工作目录 /home/jenkins/agent、添加卷选择 hostpath 把 docker、.kube 二进制文件传入使 slaver pod 里可以使用 docker 命令
# ServiceAccount 选择 jenkins，此处已在 jenkins.yaml 里编写过，填写后保存
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-10-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-11-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-12-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-13-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-14-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-15-unlock.png)
```
# 创建一个项目，效果为 查看 docker 容器、查看 kubernetes 集群 pod，构建后集群自动创建一个slaver pod，在 slaver pod 里查看，构建任务执行结束 slaver pod 消失
# 创建名称 dynamic-slave 的自由风格项目，General 选择限制项目运行节点，此处标签为 jixingxing-jnlp 就是 Add a new cloud 里添加的标签，构建步骤选择执行 shell，添加 shell 命令，然后保存
$ echo "测试 Kubernetes 动态生成 jenkins slave"
$ echo "==============docker in docker==========="
$ docker info

$ echo "=============kubectl============="
$ kubectl get pods
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-16-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-17-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-18-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-19-unlock.png)
```
# 保存后会进行项目内，选择立即构建查看构建过程，会先起一个 jnlp 的 pod，在 pod 里执行 shell 命令，显示结果，jnlp pod 自动删除
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-20-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-21-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-22-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-23-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-24-unlock.png)
- jenkins + gitlab 通过钩子自动触发构建任务
```
# jenkins 创建 pipline-webhook 流水线项目，构建触发器选择 Build when a change is pushed to GitLab，动作选择 Push Events，高级里生成 Secret token，流水线选择 Pipeline script from SCM，SCM 选择 git，添加 git 项目地址，添加密钥，选择分支，脚本路径选择 Jenkinsfile，这需要 代码库里有 Jenkinsfile 脚本
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-25-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-26-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-27-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-28-unlock.png)
```
# 把 jenkins 项目中触发器那边的 URL 与 Secret token 复制到 gitlab webhook 中，先开启 gitlab 运行 webhook 的一个前置条件后保存，进去代码库后选择 Settings->Webhook 填入 URL 与 Secret token，动作选择 Push Event，保存后选择 test 测试，返回 200 代表webhook 配置成功，同时会触发 jenkins 进行自动构建
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-29-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-30-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-31-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-32-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-33-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-34-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-35-unlock.png)
- jenkins + gitlab + Harbor + Kubernetes + helm 整套 demo
```
# jenkins 动态 slaver pod 已经测试，gitlab webhook 触发构建已经测试，一个整套 demo 就是把这些配置串起来，页面点击按钮已经不够满足需求，就需要自己编写 Jenkinsfile 脚本，启动 slaver pod，在 slaver pod 里有各种容器，在容器中执行任务
# 在 maven 容器中执行 mvn install 构建 jar 包，在 docker 容器中执行 docker build 制作镜像中后上传到 harbor，在 helm 容器中执行 helm install 安装
# 整个代码库项目目录应该如下，要有Jenkinsfile、dockerfile、pom.xml、src目录
.
├── Jenkinsfile
├── README.md
├── dockerfile
├── pom.xml
├── src
│   ├── main
│   │   ├── java
│   │   │   └── com
│   │   │       └── example
│   │   │           └── helloword
│   │   │               ├── HellowordApplication.java
│   │   │               └── controller.java
│   │   └── resources
│   │       └── application.properties
│   └── test
│       └── java
│           └── com
│               └── example
│                   └── helloword
│                       └── HellowordApplicationTests.java


# dockerfile 如下：

FROM openjdk:8-jdk-alpine

MAINTAINER jixingxing <542255405@qq.com>

ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8
ENV TZ=Asia/Shanghai

RUN mkdir /app

WORKDIR /app

COPY target/helloword-0.0.1-SNAPSHOT.jar /app

ENTRYPOINT ["java","-jar","helloword-0.0.1-SNAPSHOT.jar"]

# Jenkinsfile 如下：

def label = "slave-${UUID.randomUUID().toString()}"

def helmRepo(Map args) {
  println "添加 course repo"
  sh "helm repo add --username ${args.username} --password ${args.password} harbor  http://${args.registryurl}/chartrepo/library"

  println "更新 repo"
  sh "helm repo update"

}

def helmDeploy(Map args) {
    helmRepo(args)

    println "部署应用"
    sh "helm upgrade --install ${args.name} harbor/mychart --set image.repository=${args.image} --set image.tag=${args.tag}"
    echo "应用 ${args.name} 部署成功. 可以使用 helm status ${args.name} 查看应用状态"
  }

podTemplate(label: label, serviceAccount: 'jenkins', containers: [
  containerTemplate(name: 'maven', image: 'maven:3.6-alpine', command: 'cat', ttyEnabled: true),
  containerTemplate(name: 'docker', image: 'docker', command: 'cat', ttyEnabled: true),
  containerTemplate(name: 'helm', image: 'cnych/helm', command: 'cat', ttyEnabled: true)
], volumes: [
  hostPathVolume(mountPath: '/root/.m2', hostPath: '/var/run/m2'),
  hostPathVolume(mountPath: '/var/run/docker.sock', hostPath: '/var/run/docker.sock')
]) {
  node(label) {
    def myRepo = checkout scm
    def gitCommit = myRepo.GIT_COMMIT
    def gitBranch = myRepo.GIT_BRANCH
    def imageTag = sh(script: "git rev-parse --short HEAD", returnStdout: true).trim()
    def dockerRegistryUrl = "10.10.33.34:30001"
    def imageEndpoint = "library/helloworld"
    def image = "${dockerRegistryUrl}/${imageEndpoint}"

    stage('单元测试') {
      echo "测试阶段"
    }
    stage('代码编译打包') {
      try {
        container('maven') {
          echo "代码编译构建阶段"
          sh "mvn install"
        }
      } catch (exc) {
        println "代码编译构建失败"
        throw(exc)
      }
    }
    stage('构建 Docker 镜像') {
      withCredentials([[$class: 'UsernamePasswordMultiBinding',
        credentialsId: 'dockerhub',
        usernameVariable: 'DOCKER_HUB_USER',
        passwordVariable: 'DOCKER_HUB_PASSWORD']]) {
          container('docker') {
            echo "构建 Docker 镜像阶段"
            sh """
              docker login ${dockerRegistryUrl} -u ${DOCKER_HUB_USER} -p ${DOCKER_HUB_PASSWORD}
              docker build -t ${image}:${imageTag} .
              docker push ${image}:${imageTag}
              """
        }
      }
    }
    stage('运行 Helm') {
      withCredentials([[$class: 'UsernamePasswordMultiBinding',
        credentialsId: 'dockerhub',
        usernameVariable: 'DOCKER_HUB_USER',
        passwordVariable: 'DOCKER_HUB_PASSWORD']]) {
          container('helm') {
            // todo，也可以做一些其他的分支判断是否要直接部署
            echo "[INFO] 开始 Helm 部署"
            helmDeploy(
                name        : "helloworld",
                chartDir    : "harbor/mychart",
                registryurl : "${dockerRegistryUrl}",
                tag         : "${imageTag}",
                image       : "${image}",
                username    : "${DOCKER_HUB_USER}",
                password    : "${DOCKER_HUB_PASSWORD}"
            )
            echo "[INFO] Helm 部署应用成功..."
       }
      }
    }
  }
}


# Jenkinsfile 脚本中 docker login 用到了认证，这边可以使用 jenkins 的凭证功能，把密码存在 jenkins 凭证中，后用函数读取凭证中用户名密码，注意 ID 要与函数中对应 dockerhub
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-36-unlock.png)
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-37-unlock.png)
```
# 现在更新代码库内容，自动触发流水线构建任务，构建编译->制作镜像->上传镜像仓库->自动部署
```
![avatar](https://github.com/darianJmy/kubernetes_tools/blob/main/cicd/Photos/setup-jenkins-38-unlock.png)
