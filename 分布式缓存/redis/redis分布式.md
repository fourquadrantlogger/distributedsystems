
[book](https://gnuhpc.gitbooks.io/redis-all-about/content/HAClusterBrief/comparison.html)

+ 1.Sentinel

Redis Sentinel 是一个分布式系统， 你可以在一个架构中运行多个 Sentinel 进程（progress）， 这些进程使用流言协议（gossip protocols)来接收关于主服务器是否下线的信息， 并使用投票协议（agreement protocols）来决定是否执行自动故障迁移， 以及选择哪个从服务器作为新的主服务器.

+ 2.twemproxy

twemproxy,也叫nutcraker。是一个twtter开源的一个redis和memcache代理服务器

twemproxy

支持失败节点自动删除

可以设置重新连接该节点的时间
可以设置连接多少次之后删除该节点
该方式适合作为cache存储
支持设置HashTag

通过HashTag可以自己设定将两个KEYhash到同一个实例上去。
减少与redis的直接连接数

保持与redis的长连接
可设置代理与后台每个redis连接的数目
自动分片到后端多个redis实例上

多种hash算法（部分还没有研究明白)
可以设置后端实例的权重
避免单点问题

可以平行部署多个代理层.client自动选择可用的一个
支持redis pipelining request

支持状态监控

可设置状态监控ip和端口，访问ip和端口可以得到一个json格式的状态信息串
可设置监控信息刷新间隔时间
高吞吐量

连接复用，内存复用。
将多个连接请求，组成reids pipelining统一向redis请求。

+ 3.0 Cluster

+ Codis
