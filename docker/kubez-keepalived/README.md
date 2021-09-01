
# Kubez-keepalived


- 在node01节点创建 `kubez keepalived` 配置 `/var/lib/kubez-tools/keepalived.conf`

```
#vrrp_script 主要使keepalived监测业务判断需不需要切换vip
vrrp_script checkhaproxy
{
    script "sh /etc/keepalived/mycheckscript.sh"
    interval 3
    weight -20
}
vrrp_instance MAIN {
  state MASTER
  #ens192 可改为实际机器端口
  interface ens192
  #50 默认50即可
  virtual_router_id 50
  #200 为权重比,backup需比200低
  priority 200
  #设置了nopreempt，即使恢复也不会抢占master
  #nopreempt
  advert_int 1
  authentication {
    #认证密码默认即可
    auth_type PASS
    auth_pass pwd1
  }
  virtual_ipaddress {
    #VIP 地址
    10.10.33.35
  }
  track_script {
    checkhaproxy
  }
}
```

- 在node02节点创建 `kubez keepalived` 配置 `/var/lib/kubez-keepalived/keepalived.conf`

```
#vrrp_script 主要使keepalived监测业务判断需不需要切换vip
vrrp_script checkhaproxy
{
    script "sh /etc/keepalived/mycheckscript.sh"
    interval 3
    weight -20
}
vrrp_instance MAIN {
  state BACKUP
  #ens192 可改为实际机器端口
  interface ens192
  #50 默认50即可
  virtual_router_id 50
  #150 为权重比,backup需比200低
  priority 150
  advert_int 1
  authentication {
    #认证密码默认即可
    auth_type PASS
    auth_pass pwd1
  }
  virtual_ipaddress {
    #VIP 地址
    10.10.33.35
  }
  track_script {
    checkhaproxy
  }
}
```

- 启动代理服务
```
docker run -d --name <container_name> --privileged=true -v /var/lib/kubez-tools/:/etc/keepalived/ --net=host --pid=host jixingxing/keepalived:v0.0.1
```

- 检查服务已经正常启动
```
[root@kube02]# docker ps -a |grep jixingxing
c5cad90e9a9a        jixingxing/keepalived:v0.0.1  "/bin/sh -c 'service…"   14 seconds ago      Up 13 seconds                      amazing_zhukovsky
```
