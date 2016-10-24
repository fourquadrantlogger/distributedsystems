# 一致性hash

 当一个缓存服务由多个服务器组共同提供时, key应该路由到哪一个服务.这里假如采用最通用的方式key%N(N为服务器数目), 这里乍一看没什么问题, 但是当服务器数目发送增加或减少时, 分配方式则变为key%(N+1)或key%(N-1).这里将会有大量的key失效迁移,如果后端key对应的是有状态的存储数据,那么毫无疑问,这种做法将导致服务器间大量的数据迁移,从而照成服务的不稳定. 为了解决类问题,一致性hash算法应运而生.
 
## 要求

在分布式缓存中, 一个好的hash算法应该要满足以下几个条件:

+ 均衡性(Balance)
       均衡性主要指,通过算法分配, 集群中各节点应该要尽可能均衡.
       
+  单调性(Monotonicity)
       单调性主要指当集群发生变化时, 已经分配到老节点的key, 尽可能的任然分配到之前节点,以防止大量数据迁移, 这里一般的hash取模就很难满足这点,而一致性hash算法能够将发生迁移的key数量控制在较低的水平.
       
+ 分散性(Spread)
        分散性主要针对同一个key, 当在不同客户端操作时,可能存在客户端获取到的缓存集群的数量不一致,从而导致将key映射到不同节点的问题,这会引发数据的不一致性.好的hash算法应该要尽可能避免分散性.
        
+ 负载(Load)
     负载主要是针对一个缓存而言, 同一缓存有可能会被用户映射到不同的key上,从而导致该缓存的状态不一致.

从原理来看,一致性hash算法针对以上问题均有一个合理的解决

## 算法流程

一致性hash的核心思想为将key作hash运算, 并按一定规律取整得出0-2^32-1之间的值, 环的大小为2^32，key计算出来的整数值则为key在hash环上的位置，如何将一个key，映射到一个节点， 这里分为两步.

第一步, 将服务的key按该hash算法计算,得到在服务在一致性hash环上的位置.
第二步, 将缓存的key，用同样的方法计算出hash环上的位置，按顺时针方向，找到第一个大于等于该hash环位置的服务key，从而得到该key需要分配的服务器。
![](/img/一致性hash.jpg)

```
//C为client客户端，S为service服务器
public class ConsistentHash<c,s> {
    //虚拟节点的个数，默认200
    private int virtualNum;
    private long unit;//移动
    //存储服务器的容器
    private TreeMap<integer,s> map = new TreeMap<>();
    public ConsistentHash() {
        this(200);
    }
    public ConsistentHash(int virtualNum) {
        this.virtualNum = virtualNum;
        unit = ((long)(Integer.MAX_VALUE/virtualNum)<<1)-2;
    }
    //添加服务器结点
    public void add(S s){
        int hash = hash(s);
        //保证分散到Integer的范围内
        for (int i = 0; i < virtualNum; i++) 
            map.put((int)(hash+i*unit), s);
    }
    //删除服务器结点
    public void remove(S s){
        int hash = hash(s);
        //保证分散到Integer的范围内,先转换为int再转换为Integer
        for (int i = 0; i < virtualNum; i++) 
            map.remove((Integer)(int)(hash+i*unit));
    }
    //获取服务器
    public S get(C c){
        if(map.isEmpty())
            return null;
        int hash = hash(c);
        //TreeMap独有的取上和取下方法
        //ceiling 和 floor (Math中是ceil)
        Entry<integer,s> entry = map.ceilingEntry(hash);
        if(entry == null) //空就代表需要回环了
            return map.firstEntry().getValue();
        else
            return entry.getValue();
    }
    //再次分散的hash方法
    private int hash(Object o) {
        if(o == null)
            return 0;
        int h = 0;
        h ^= o.hashCode();
        h ^= (h >>> 20) ^ (h >>> 12);
        return h ^ (h >>> 7) ^ (h >>> 4);
    }
}

```

首先求出服务器的哈希值， 并将其配置到0～2^32的圆上。 然后用同样的方法求出客户端的哈希值，并映射到圆上。 然后从数据映射到的位置开始顺时针查找，第一个服务器服务它。 如果超过2^32找不到服务器，就会保存到第一台服务器上。
使用虚拟节点的思想，为每个服务器在圆上分配100～200个点。这样就能抑制分布不均匀， 最大限度地减小服务器增减时的缓存重新分布。