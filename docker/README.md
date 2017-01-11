## 删除容器

```
sudo docker rm -f  `sudo docker ps -a -q`
```

## 删除空镜像
```
sudo docker rmi -f `sudo docker images | awk '/^<none>/ { print $3 }'`
```
