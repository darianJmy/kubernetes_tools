# Kubez-haproxy


- 在node01节点创建 `kubez haproxy` 配置 `/var/lib/kubez-tools/haproxy.cfg`, 只需配置`listen`

```
listen mariadb
   bind 10.10.33.35:3306
   mode tcp
   server mariadb01 10.10.33.32:3306 maxconn 32
   server mariadb02 10.10.33.33:3306 maxconn 32

```

- 启动代理服务
```
docker run -d --name <container_name> --privileged=true -v /var/lib/kubez-tools/:/etc/haproxy/ --net=host 10.10.33.31:5000/jixingxing/kubez-haproxy:v0.0.4
```

- 检查服务已经正常启动
```
[root@kube02 ~]# docker ps -a |grep jixingxing
3b2ceb6ff93e        10.10.33.31:5000/jixingxing/kubez-haproxy:v0.0.4  "/usr/sbin/haproxy -…"   4 minutes ago  Up 4 minutes      haproxy
```

 - 在node02节点重复创建 `kubez haproxy` 操作
