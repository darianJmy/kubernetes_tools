
# Kubez-keepalived


- 在master节点创建 `kubez keepalived` 配置 `/var/lib/kubez-keepalived/keepalived.conf`

```
vrrp_instance MAIN {
  state MASTER
  #ens192 可改为实际机器端口
  interface ens192
  #50 默认50即可
  virtual_router_id 50
  #200 为权重比,backup需比200低
  priority 200
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
}
```

- 在backup节点创建 `kubez keepalived` 配置 `/var/lib/kubez-keepalived/keepalived.conf`

```
vrrp_instance MAIN {
  state BACKUP
  #ens192 可改为实际机器端口
  interface ens192
  #50 默认50即可
  virtual_router_id 50
  #200 为权重比,backup需比200低
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
}
```

- 启动代理服务
```
docker run -d --name <container_name> --privileged=true -v /etc/keepalived/keepalived.conf:/etc/keepalived/keepalived.conf --net=host jixingxing/keepalived:v0.0.1
```

- 检查服务已经正常启动
```
[root@kube02]# docker ps -a |grep jixingxing
c5cad90e9a9a        jixingxing/keepalived:v0.0.1                                    "/bin/sh -c 'service…"   14 seconds ago      Up 13 seconds                                 amazing_zhukovsky
```
