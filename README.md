# 项目介绍

本项目用于创建一个实验性质的HTTP服务器，仅可用于学习
https://github.com/lanceliuGithub/cncamp_ch10_homework.git

# 编译二进制可执行文件

建议在 Linux 环境运行如下编译命令，Windows平台请先安装 Cygwin
```
make
```
或
```
make build
```

# 应用配置说明

手工编译代码后，应用的二进制会输出到 bin/linux/amd64 目录下
```
bin
└── linux
    └── amd64
        ├── config.json
        └── myhttpserver-1.0
```

同时在相同目录下会生成一份默认配置文件 config.json
```
{
	"server": {
		"host": "0.0.0.0",
		"port": "8888"
	},
	"log": {
		"enable": true,
		"request_header": false
	}
}
```

其中 server.host 是服务器监听的主机，server.port 是服务器监听的端口

log.enable 是记录后台日志的总开关，开启后日志会直接打印在控制台中，默认开启

log.request_header 是细化的日志开关（只有 log.enable 为 true 时才生效），此选项默认关闭

# 应用启动说明

查看启动选项
```
./myhttpserver-1.0 -h
Usage of ./myhttpserver-1.0:
  -c string
    	Specify an alternative config file (default "config.json")
```

目前只有一个选项 -c ，用于指定不同的配置文件供服务器加载
```
./myhttpserver-1.0 -c /etc/another_config.json
```

本HTTP服务器启动后，会模拟两个阶段的耗时
1. 启动耗时，共5s
2. 服务就绪耗时，共10s

启动耗时是从应用启动后，到端口被侦听这段时间，耗时5s

服务器就绪耗时是等启动耗时过去后，再等5s，/healthz接口才能返回成功，否则返回500状态码和failed包体

# 制作容器镜像

生成容器镜像
```
make release
```
请注意release动作包括了make，只不过编译动作是在容器中完成的。
如果只想单独编译出二进制，请使用 make build

生成容器镜像并推送到 Docker Hub 公开仓库
```
make push
```

如果推送时报错 `denied: requested access to the resource is denied` ，请先登录 docker.com
```
docker login
```

# 使用K8S优雅管理一个Pod

配置文件位于 k8s-plan/graceful-pod.yaml

运行如下命令
```
kubectl apply -f k8s-plan/graceful-pod.yaml
```

观察Pod的状态变化
```
kubectl get pod myhttpserver -w
```

查看HTTP服务器后台日志
```
kubectl logs -f myhttpserver
```


在宿主机上访问HTTP服务

- 首页: http://localhost
- 健康检查页: http://localhost/healthz
- 缺失的页面: http://localhost/no_such_page

移除应用
```
kubectl delete -f k8s-plan/graceful-pod.yaml
```

# 使用K8S维护一个安全且高可用的服务

配置文件位于 k8s-plan/secure-ha-service 目录下

部署所有对象
```
kubectl apply \
  -f 1.config.yaml \
  -f 2.deploy.yaml \
  -f 3.service.yaml \
  -f 4.ingress-nginx-deploy.yaml \
  -f 5.ingress-cert.yaml \
  -f 6.ingress.yaml
```

卸载所有对象
```
kubectl delete \
  -f 6.ingress.yaml \
  -f 5.ingress-cert.yaml \
  -f 4.ingress-nginx-deploy.yaml \
  -f 3.service.yaml \
  -f 2.deploy.yaml \
  -f 1.config.yaml
```

注意：卸载时，如果报如下错误，可以稍等一会再试
```
Error from server (InternalError): error when creating "6.ingress.yaml": Internal error occurred: failed calling webhook "validate.nginx.ingress.kubernetes.io": Post "https://ingress-nginx-controller-admission.ingress-nginx.svc:443/networking/v1/ingresses?timeout=10s": dial tcp 10.105.108.221:443: connect: connection refused
```

发起HTTP访问
```
GATEWAY=`kubectl get -n ingress-nginx svc ingress-nginx-controller -ojson | jq -r '.spec.clusterIP'`
curl -k -H "Host: lancelot.cn" https://$GATEWAY/healthz
```

对象yaml说明：
- 1.config.yaml   HTTP服务器的配置文件对象（ConfigMap）
- 2.deploy.yaml   HTTP服务器的部署对象（Deployment）
- 3.service.yaml  HTTP服务器的服务对象（Service）
- 4.ingress-nginx-deploy.yaml   Nginx实现的Ingress控制器
- 5.ingress-cert.yaml   HTTP服务器的TLS证书（Secret TLS）
- 6.ingress.yaml  HTTP服务器的网关对象（Ingress）

重新生成证书使用命令并同时修改 5.ingress-cert.yaml：
```
cd k8s-plan/secure-ha-service

openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
-subj "/CN=lancelot.cn/O=lancelot" \
-addext "subjectAltName = DNS:lancelot.cn" \
-keyout lancelot_cn.key -out lancelot_cn.crt
```

# 模拟监控

## 为 HTTPServer 添加 0-2 秒的随机延时

见 myhttpserver.go
```
func handleRoot(w http.ResponseWriter, r *http.Request) {
  ...
  delayMillisecs := randInt(10,2000)
  delay := time.Millisecond * time.Duration(delayMillisecs)
  time.Sleep(delay)
  ...
}

func randInt(min int, max int) int {
  rand.Seed(time.Now().UTC().UnixNano())
  return min + rand.Intn(max-min)
}
```

## 为 HTTPServer 项目添加延时 Metric

见 myhttpserver.go 和 metrics/metrics.go
引入了 prometheus 依赖 github.com/prometheus/client_golang/prometheus/promhttp
```
func main() {
  startTime = time.Now()
  metrics.Register()
  ...
	http.Handle("/metrics", promhttp.Handler())
	...
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	...
  timer := metrics.NewTimer()
  defer timer.ObserveTotal()
	...
}
```

## 将 HTTPServer 部署至测试集群，并完成 Prometheus 配置

安装 Helm
```
curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg > /dev/null
apt-get install apt-transport-https --yes
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list
apt-get update
apt-get install helm
```

安装 Loki 和 Grafana
```
helm repo add grafana https://grafana.github.io/helm-charts
helm upgrade --install loki grafana/loki-stack --set grafana.enabled=true,prometheus.enabled=true,prometheus.alertmanager.persistentVolume.enabled=false,prometheus.server.persistentVolume.enabled=false
```

修改 Grafana Service 类型为 NodePort
```
kubectl patch svc loki-grafana --type='json' -p '[{"op":"replace","path":"/spec/type","value":"NodePort"},{"op":"replace","path":"/spec/ports/0/nodePort","value":30066}]'
```

获取 Grafana 的用户名和口令
```
kubectl get secret loki-grafana -ojson | jq -r '.data."admin-user"' | base64 -d
kubectl get secret loki-grafana -ojson | jq -r '.data."admin-password"' | base64 -d
```

## 从 Promethus 界面中查询延时指标数据

生成测试用的延迟指标数据
```
GATEWAY=`kubectl get -n ingress-nginx svc ingress-nginx-controller -ojson | jq -r '.spec.clusterIP'`
for i in {1..100}; do curl -k -H "Host: lancelot.cn" https://$GATEWAY; done
```

查看实时指标数据
```
curl -s -k -H "Host: lancelot.cn" https://$GATEWAY/metrics | grep execution_latency_seconds
```

## 创建一个 Grafana Dashboard 展现延时分配情况

配置 Grafana 监控
```
histogram_quantile(0.95, sum(rate(default_execution_latency_seconds_bucket[5m])) by (le))
histogram_quantile(0.90, sum(rate(default_execution_latency_seconds_bucket[5m])) by (le))
histogram_quantile(0.50, sum(rate(default_execution_latency_seconds_bucket[5m])) by (le))
```
