# redis优化建议

https://gnuhpc.gitbooks.io/redis-all-about/content/CodeDesignRule/latency.html

尽可能使用pipeline操作：一次性的发送命令比一个个发要减少网络延迟和单个处理开销。

3. 对于数据量较大的集合，不要轻易进行删除操作，这样会阻塞服务器，一般采用重命名+批量删除的策略：